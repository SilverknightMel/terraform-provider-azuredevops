package build

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/client"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/model"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/utils"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/utils/converter"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/utils/tfhelper"
)

const (
	bqVariable              = "variable"
	bqVariableName          = "name"
	bqVariableValue         = "value"
	bqSecretVariableValue   = "secret_value"
	bqVariableIsSecret      = "is_secret"
	bqVariableAllowOverride = "allow_override"
)

// ResourceBuildQueue schema and implementation for build definition resource
func ResourceBuildQueue() *schema.Resource {
	filterSchema := map[string]*schema.Schema{
		"include": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type:         schema.TypeString,
				ValidateFunc: validation.NoZeroValues,
			},
		},
		"exclude": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type:         schema.TypeString,
				ValidateFunc: validation.NoZeroValues,
			},
		},
	}



	return &schema.Resource{
		Create:   ResourceBuildQueueCreate,
		Read:     ResourceBuildQueueRead,
		Update:   ResourceBuildQueueUpdate,
		Delete:   ResourceBuildQueueDelete,
		Importer: tfhelper.ImportProjectQualifiedResource(),
		Schema: map[string]*schema.Schema{
			"project_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"Build": {
				Type:     schema.TypeInt,
				Required: false,
				Computed: true,
			},
			"IgnoreWarnings": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  "",
			},
			"CheckInTicket": {
				Type:         schema.TypeString,
				Optional:     true,
			},
			"SourceBuildId": {
				Type:     schema.TypeInt,
				Optional: true,

			},
			bqVariable: {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						bqVariableName: {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotWhiteSpace,
						},
						bqVariableValue: {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						bqSecretVariableValue: {
							Type:      schema.TypeString,
							Optional:  true,
							Sensitive: true,
							Default:   "",
						},
						bqVariableIsSecret: {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						bqVariableAllowOverride: {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},
					},
				},
			},
		},
	}
}


func ResourceBuildQueueCreate(d *schema.ResourceData, m interface{}) error {
	clients := m.(*client.AggregatedClient)
	err := validateServiceConnectionIDExistsIfNeeded(d)
	if err != nil {
		return err
	}
	BuildQueue, projectID, err := expandBuildQueue(d)
	if err != nil {
		return fmt.Errorf("error creating resource Build Definition: %+v", err)
	}

	createdBuildQueue, err := createBuildQueue(clients, BuildQueue, projectID)
	if err != nil {
		return fmt.Errorf("error creating resource Build Definition: %+v", err)
	}

//	flattenBuildQueue(d, createdBuildQueue, projectID)
	return ResourceBuildQueueRead(d, m)
}

func flattenBuildQueue(d *schema.ResourceData, BuildQueue *build.BuildQueue, projectID string) {
	d.SetId(strconv.Itoa(*BuildQueue.Id))

	d.Set("project_id", projectID)
	d.Set("name", *BuildQueue.Name)
	d.Set("path", *BuildQueue.Path)
	d.Set("repository", flattenRepository(BuildQueue))

	if BuildQueue.Queue != nil && BuildQueue.Queue.Pool != nil {
		d.Set("agent_pool_name", *BuildQueue.Queue.Pool.Name)
	}

	d.Set("variable_groups", flattenVariableGroups(BuildQueue))
	d.Set(bqVariable, flattenBuildVariables(d, BuildQueue))

	if BuildQueue.Triggers != nil {
		yamlCiTrigger := hasSettingsSourceType(BuildQueue.Triggers, build.DefinitionTriggerTypeValues.ContinuousIntegration, 2)
		d.Set("ci_trigger", flattenBuildQueueTriggers(BuildQueue.Triggers, yamlCiTrigger, build.DefinitionTriggerTypeValues.ContinuousIntegration))

		yamlPrTrigger := hasSettingsSourceType(BuildQueue.Triggers, build.DefinitionTriggerTypeValues.PullRequest, 2)
		d.Set("pull_request_trigger", flattenBuildQueueTriggers(BuildQueue.Triggers, yamlPrTrigger, build.DefinitionTriggerTypeValues.PullRequest))
	}

	revision := 0
	if BuildQueue.Revision != nil {
		revision = *BuildQueue.Revision
	}

	d.Set("revision", revision)
}

// Return an interface suitable for serialization into the resource state. This function ensures that
// any secrets, for which values will not be returned by the service, are not overidden with null or
// empty values
func flattenBuildVariables(d *schema.ResourceData, BuildQueue *build.BuildQueue) interface{} {
	if BuildQueue.Variables == nil {
		return nil
	}
	variables := make([]map[string]interface{}, len(*BuildQueue.Variables))

	index := 0
	for varName, varVal := range *BuildQueue.Variables {
		var variable map[string]interface{}

		isSecret := converter.ToBool(varVal.IsSecret, false)
		variable = map[string]interface{}{
			bqVariableName:          varName,
			bqVariableValue:         converter.ToString(varVal.Value, ""),
			bqVariableIsSecret:      isSecret,
			bqVariableAllowOverride: converter.ToBool(varVal.AllowOverride, false),
		}

		//read secret variable from state if exist
		if isSecret {
			if stateVal := tfhelper.FindMapInSetWithGivenKeyValue(d, bqVariable, bqVariableName, varName); stateVal != nil {
				variable = stateVal
			}
		}
		variables[index] = variable
		index = index + 1
	}

	return variables
}

func createBuildQueue(clients *client.AggregatedClient, BuildQueue *build.BuildQueue, project string) (*build.BuildQueue, error) {
	createdBuild, err := clients.BuildClient.CreateDefinition(clients.Ctx, build.CreateDefinitionArgs{
		Definition: BuildQueue,
		Project:    &project,
	})

	return createdBuild, err
}

func ResourceBuildQueueRead(d *schema.ResourceData, m interface{}) error {
	clients := m.(*client.AggregatedClient)
	projectID, BuildQueueID, err := tfhelper.ParseProjectIDAndResourceID(d)

	if err != nil {
		return err
	}

	BuildQueue, err := clients.BuildClient.GetDefinition(clients.Ctx, build.GetDefinitionArgs{
		Project:      &projectID,
		DefinitionId: &BuildQueueID,
	})

	if err != nil {
		if utils.ResponseWasNotFound(err) {
			d.SetId("")
			return nil
		}
		return err
	}

	//flattenBuildQueue(d, BuildQueue, projectID)
	return nil
}

func ResourceBuildQueueDelete(d *schema.ResourceData, m interface{}) error {
	if strings.EqualFold(d.Id(), "") {
		return nil
	}

	clients := m.(*client.AggregatedClient)
	projectID, BuildQueueID, err := tfhelper.ParseProjectIDAndResourceID(d)
	if err != nil {
		return err
	}

	err = clients.BuildClient.DeleteDefinition(m.(*client.AggregatedClient).Ctx, build.DeleteDefinitionArgs{
		Project:      &projectID,
		DefinitionId: &BuildQueueID,
	})

	return err
}

func ResourceBuildQueueUpdate(d *schema.ResourceData, m interface{}) error {
	clients := m.(*client.AggregatedClient)
	err := validateServiceConnectionIDExistsIfNeeded(d)
	if err != nil {
		return err
	}
	BuildQueue, projectID, err := expandBuildQueue(d)
	if err != nil {
		return err
	}

	updatedBuildQueue, err := clients.BuildClient.UpdateDefinition(m.(*client.AggregatedClient).Ctx, build.UpdateDefinitionArgs{
		Definition:   BuildQueue,
		Project:      &projectID,
		DefinitionId: BuildQueue.Id,
	})

	if err != nil {
		return err
	}

	//flattenBuildQueue(d, updatedBuildQueue, projectID)
	return ResourceBuildQueueRead(d, m)
}

func flattenVariableGroups(BuildQueue *build.BuildQueue) []int {
	if BuildQueue.VariableGroups == nil {
		return nil
	}

	variableGroups := make([]int, len(*BuildQueue.VariableGroups))

	for i, variableGroup := range *BuildQueue.VariableGroups {
		variableGroups[i] = *variableGroup.Id
	}

	return variableGroups
}

func flattenRepository(BuildQueue *build.BuildQueue) interface{} {
	yamlFilePath := ""
	githubEnterpriseUrl := ""

	// The process member can be of many types -- the only typing information
	// available from the compiler is `interface{}` so we can probe for known
	// implementations
	if processMap, ok := BuildQueue.Process.(map[string]interface{}); ok {
		yamlFilePath = processMap["yamlFilename"].(string)
	}
	if yamlProcess, ok := BuildQueue.Process.(*build.YamlProcess); ok {
		yamlFilePath = *yamlProcess.YamlFilename
	}

	// Set github_enterprise_url value from BuildQueue.Repository URL
	if strings.EqualFold(*BuildQueue.Repository.Type, string(model.RepoTypeValues.GitHubEnterprise)) {
		url, err := url.Parse(*BuildQueue.Repository.Url)
		if err != nil {
			return fmt.Errorf("Unable to parse repository URL: %+v ", err)
		}
		githubEnterpriseUrl = fmt.Sprintf("%s://%s", url.Scheme, url.Host)
	}

	reportBuildStatus, err := strconv.ParseBool((*BuildQueue.Repository.Properties)["reportBuildStatus"])
	if err != nil {
		return fmt.Errorf("Unable to parse `reportBuildStatus` property: %+v ", err)
	}
	return []map[string]interface{}{{
		"yml_path":              yamlFilePath,
		"repo_id":               *BuildQueue.Repository.Id,
		"repo_type":             *BuildQueue.Repository.Type,
		"branch_name":           *BuildQueue.Repository.DefaultBranch,
		"service_connection_id": (*BuildQueue.Repository.Properties)["connectedServiceId"],
		"github_enterprise_url": githubEnterpriseUrl,
		"report_build_status":   reportBuildStatus,
	}}
}

func flattenBuildQueueBranchOrPathFilter(m []interface{}) []interface{} {
	var include []string
	var exclude []string

	for _, v := range m {
		if v2, ok := v.(string); ok {
			if strings.HasPrefix(v2, "-") {
				exclude = append(exclude, strings.TrimPrefix(v2, "-"))
			} else if strings.HasPrefix(v2, "+") {
				include = append(include, strings.TrimPrefix(v2, "+"))
			}
		}
	}

	return []interface{}{
		map[string]interface{}{
			"include": include,
			"exclude": exclude,
		},
	}
}

func flattenBuildQueueContinuousIntegrationTrigger(m interface{}, isYaml bool) interface{} {
	if ms, ok := m.(map[string]interface{}); ok {
		f := map[string]interface{}{
			"use_yaml": isYaml,
		}
		if !isYaml {
			f["override"] = []map[string]interface{}{{
				"batch":                            ms["batchChanges"],
				"branch_filter":                    flattenBuildQueueBranchOrPathFilter(ms["branchFilters"].([]interface{})),
				"max_concurrent_builds_per_branch": ms["maxConcurrentBuildsPerBranch"],
				"polling_interval":                 ms["pollingInterval"],
				"polling_job_id":                   ms["pollingJobId"],
				"path_filter":                      flattenBuildQueueBranchOrPathFilter(ms["pathFilters"].([]interface{})),
			}}
		}
		return f
	}
	return nil
}

func flattenBuildQueuePullRequestTrigger(m interface{}, isYaml bool) interface{} {
	if ms, ok := m.(map[string]interface{}); ok {
		forks := ms["forks"].(map[string]interface{})
		isCommentRequired := ms["isCommentRequiredForPullRequest"].(bool)
		isCommentRequiredNonTeam := ms["requireCommentsForNonTeamMembersOnly"].(bool)

		var commentRequired string
		if isCommentRequired {
			commentRequired = "All"
		}
		if isCommentRequired && isCommentRequiredNonTeam {
			commentRequired = "NonTeamMembers"
		}

		branchFilters := ms["branchFilters"].([]interface{})
		var initialBranch string
		if len(branchFilters) > 0 {
			initialBranch = strings.TrimPrefix(branchFilters[0].(string), "+")
		}

		f := map[string]interface{}{
			"use_yaml":         isYaml,
			"initial_branch":   initialBranch,
			"comment_required": commentRequired,
			"forks": []map[string]interface{}{{
				"enabled":       forks["enabled"],
				"share_secrets": forks["allowSecrets"],
			}},
		}
		if !isYaml {
			f["override"] = []map[string]interface{}{{
				"auto_cancel":   ms["autoCancel"],
				"branch_filter": flattenBuildQueueBranchOrPathFilter(branchFilters),
				"path_filter":   flattenBuildQueueBranchOrPathFilter(ms["pathFilters"].([]interface{})),
			}}
		}
		return f
	}
	return nil
}

func flattenBuildQueueTrigger(m interface{}, isYaml bool, t build.DefinitionTriggerType) interface{} {
	if ms, ok := m.(map[string]interface{}); ok {
		if ms["triggerType"].(string) != string(t) {
			return nil
		}
		switch t {
		case build.DefinitionTriggerTypeValues.ContinuousIntegration:
			return flattenBuildQueueContinuousIntegrationTrigger(ms, isYaml)
		case build.DefinitionTriggerTypeValues.PullRequest:
			return flattenBuildQueuePullRequestTrigger(ms, isYaml)
		}
	}
	return nil
}

func hasSettingsSourceType(m *[]interface{}, t build.DefinitionTriggerType, sst int) bool {
	hasSetting := false
	for _, d := range *m {
		if ms, ok := d.(map[string]interface{}); ok {
			if strings.EqualFold(ms["triggerType"].(string), string(t)) {
				if val, ok := ms["settingsSourceType"]; ok {
					hasSetting = int(val.(float64)) == sst
				}
			}
		}
	}
	return hasSetting
}

func flattenBuildQueueTriggers(m *[]interface{}, isYaml bool, t build.DefinitionTriggerType) []interface{} {
	ds := make([]interface{}, 0, len(*m))
	for _, d := range *m {
		f := flattenBuildQueueTrigger(d, isYaml, t)
		if f != nil {
			ds = append(ds, f)
		}
	}
	return ds
}

func expandBuildQueueBranchOrPathFilter(d map[string]interface{}) []interface{} {
	include := tfhelper.ExpandStringSet(d["include"].(*schema.Set))
	exclude := tfhelper.ExpandStringSet(d["exclude"].(*schema.Set))
	m := make([]interface{}, len(include)+len(exclude))
	i := 0
	for _, v := range include {
		m[i] = "+" + v
		i++
	}
	for _, v := range exclude {
		m[i] = "-" + v
		i++
	}
	return m
}
func expandBuildQueueBranchOrPathFilterList(d []interface{}) [][]interface{} {
	vs := make([][]interface{}, 0, len(d))
	for _, v := range d {
		if val, ok := v.(map[string]interface{}); ok {
			vs = append(vs, expandBuildQueueBranchOrPathFilter(val))
		}
	}
	return vs
}
func expandBuildQueueBranchOrPathFilterSet(configured *schema.Set) []interface{} {
	d2 := expandBuildQueueBranchOrPathFilterList(configured.List())
	if len(d2) != 1 {
		return nil
	}
	return d2[0]
}

func expandBuildQueueFork(d map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"allowSecrets": d["share_secrets"].(bool),
		"enabled":      d["enabled"].(bool),
	}
}
func expandBuildQueueForkList(d []interface{}) []map[string]interface{} {
	vs := make([]map[string]interface{}, 0, len(d))
	for _, v := range d {
		if val, ok := v.(map[string]interface{}); ok {
			vs = append(vs, expandBuildQueueFork(val))
		}
	}
	return vs
}
func expandBuildQueueForkListFirstOrNil(d []interface{}) map[string]interface{} {
	d2 := expandBuildQueueForkList(d)
	if len(d2) != 1 {
		return nil
	}
	return d2[0]
}

func expandBuildQueueManualPullRequestTrigger(d map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"branchFilters": expandBuildQueueBranchOrPathFilterSet(d["branch_filter"].(*schema.Set)),
		"pathFilters":   expandBuildQueueBranchOrPathFilterSet(d["path_filter"].(*schema.Set)),
		"autoCancel":    d["auto_cancel"].(bool),
	}
}
func expandBuildQueueManualPullRequestTriggerList(d []interface{}) []map[string]interface{} {
	vs := make([]map[string]interface{}, 0, len(d))
	for _, v := range d {
		if val, ok := v.(map[string]interface{}); ok {
			vs = append(vs, expandBuildQueueManualPullRequestTrigger(val))
		}
	}
	return vs
}
func expandBuildQueueManualPullRequestTriggerListFirstOrNil(d []interface{}) map[string]interface{} {
	d2 := expandBuildQueueManualPullRequestTriggerList(d)
	if len(d2) != 1 {
		return nil
	}
	return d2[0]
}

func expandBuildQueueManualContinuousIntegrationTrigger(d map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"batchChanges":                 d["batch"].(bool),
		"branchFilters":                expandBuildQueueBranchOrPathFilterSet(d["branch_filter"].(*schema.Set)),
		"maxConcurrentBuildsPerBranch": d["max_concurrent_builds_per_branch"].(int),
		"pathFilters":                  expandBuildQueueBranchOrPathFilterSet(d["path_filter"].(*schema.Set)),
		"triggerType":                  string(build.DefinitionTriggerTypeValues.ContinuousIntegration),
		"pollingInterval":              d["polling_interval"].(int),
	}
}
func expandBuildQueueManualContinuousIntegrationTriggerList(d []interface{}) []map[string]interface{} {
	vs := make([]map[string]interface{}, 0, len(d))
	for _, v := range d {
		if val, ok := v.(map[string]interface{}); ok {
			vs = append(vs, expandBuildQueueManualContinuousIntegrationTrigger(val))
		}
	}
	return vs
}
func expandBuildQueueManualContinuousIntegrationTriggerListFirstOrNil(d []interface{}) map[string]interface{} {
	d2 := expandBuildQueueManualContinuousIntegrationTriggerList(d)
	if len(d2) != 1 {
		return nil
	}
	return d2[0]
}

func expandBuildQueueTrigger(d map[string]interface{}, t build.DefinitionTriggerType) interface{} {
	switch t {
	case build.DefinitionTriggerTypeValues.ContinuousIntegration:
		isYaml := d["use_yaml"].(bool)
		if isYaml {
			return map[string]interface{}{
				"batchChanges":                 false,
				"branchFilters":                []interface{}{},
				"maxConcurrentBuildsPerBranch": 1,
				"pathFilters":                  []interface{}{},
				"triggerType":                  string(t),
				"settingsSourceType":           float64(2),
			}
		}
		return expandBuildQueueManualContinuousIntegrationTriggerListFirstOrNil(d["override"].([]interface{}))
	case build.DefinitionTriggerTypeValues.PullRequest:
		isYaml := d["use_yaml"].(bool)
		commentRequired := d["comment_required"].(string)
		vs := map[string]interface{}{
			"forks":                                expandBuildQueueForkListFirstOrNil(d["forks"].([]interface{})),
			"isCommentRequiredForPullRequest":      len(commentRequired) > 0,
			"requireCommentsForNonTeamMembersOnly": commentRequired == "NonTeamMembers",
			"triggerType":                          string(t),
		}
		if isYaml {
			vs["branchFilters"] = []interface{}{
				"+" + d["initial_branch"].(string),
			}
			vs["pathFilters"] = []interface{}{}
			vs["settingsSourceType"] = float64(2)
		} else {
			override := expandBuildQueueManualPullRequestTriggerListFirstOrNil(d["override"].([]interface{}))
			vs["branchFilters"] = override["branchFilters"]
			vs["pathFilters"] = override["pathFilters"]
			vs["autoCancel"] = override["autoCancel"]
		}
		return vs
	}
	return nil
}
func expandBuildQueueTriggerList(d []interface{}, t build.DefinitionTriggerType) []interface{} {
	vs := make([]interface{}, 0, len(d))
	for _, v := range d {
		val, ok := v.(map[string]interface{})
		if ok {
			vs = append(vs, expandBuildQueueTrigger(val, t))
		}
	}
	return vs
}

func expandVariableGroups(d *schema.ResourceData) *[]build.VariableGroup {
	variableGroupsInterface := d.Get("variable_groups").(*schema.Set).List()
	variableGroups := make([]build.VariableGroup, len(variableGroupsInterface))

	for i, variableGroup := range variableGroupsInterface {
		variableGroups[i] = *buildVariableGroup(variableGroup.(int))
	}

	return &variableGroups
}

func expandVariables(d *schema.ResourceData) (*map[string]build.BuildQueueVariable, error) {
	variables := d.Get(bqVariable)
	if variables == nil {
		return nil, nil
	}

	variablesList := variables.(*schema.Set).List()
	if len(variablesList) == 0 {
		return nil, nil
	}

	expandedVars := map[string]build.BuildQueueVariable{}
	for _, variable := range variablesList {
		varAsMap := variable.(map[string]interface{})
		varName := varAsMap[bqVariableName].(string)

		if _, ok := expandedVars[varName]; ok {
			return nil, fmt.Errorf("Unexpectedly found duplicate variable with name %s", varName)
		}

		isSecret := converter.Bool(varAsMap[bqVariableIsSecret].(bool))
		var val *string

		if *isSecret {
			val = converter.String(varAsMap[bqSecretVariableValue].(string))
		} else {
			val = converter.String(varAsMap[bqVariableValue].(string))
		}
		expandedVars[varName] = build.BuildQueueVariable{
			AllowOverride: converter.Bool(varAsMap[bqVariableAllowOverride].(bool)),
			IsSecret:      isSecret,
			Value:         val,
		}
	}

	return &expandedVars, nil
}

func expandBuildQueue(d *schema.ResourceData) (*build.BuildQueue, string, error) {
	projectID := d.Get("project_id").(string)
	repositories := d.Get("repository").([]interface{})

	// Note: If configured, this will be of length 1 based on the schema definition above.
	if len(repositories) != 1 {
		return nil, "", fmt.Errorf("Unexpectedly did not find repository metadata in the resource data")
	}

	repository := repositories[0].(map[string]interface{})

	repoID := repository["repo_id"].(string)
	repoType := model.RepoType(repository["repo_type"].(string))
	repoURL := ""
	repoAPIURL := ""

	if strings.EqualFold(string(repoType), string(model.RepoTypeValues.GitHub)) {
		repoURL = fmt.Sprintf("https://github.com/%s.git", repoID)
		repoAPIURL = fmt.Sprintf("https://api.github.com/repos/%s", repoID)
	}
	if strings.EqualFold(string(repoType), string(model.RepoTypeValues.Bitbucket)) {
		repoURL = fmt.Sprintf("https://bitbucket.org/%s.git", repoID)
		repoAPIURL = fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s", repoID)
	}
	if strings.EqualFold(string(repoType), string(model.RepoTypeValues.GitHubEnterprise)) {
		githubEnterpriseURL := repository["github_enterprise_url"].(string)
		repoURL = fmt.Sprintf("%s/%s.git", githubEnterpriseURL, repoID)
		repoAPIURL = fmt.Sprintf("%s/api/v3/repos/%s", githubEnterpriseURL, repoID)
	}

	ciTriggers := expandBuildQueueTriggerList(
		d.Get("ci_trigger").([]interface{}),
		build.DefinitionTriggerTypeValues.ContinuousIntegration,
	)
	pullRequestTriggers := expandBuildQueueTriggerList(
		d.Get("pull_request_trigger").([]interface{}),
		build.DefinitionTriggerTypeValues.PullRequest,
	)

	buildTriggers := append(ciTriggers, pullRequestTriggers...)

	// Look for the ID. This may not exist if we are within the context of a "create" operation,
	// so it is OK if it is missing.
	BuildQueueID, err := strconv.Atoi(d.Id())
	var BuildQueueReference *int
	if err == nil {
		BuildQueueReference = &BuildQueueID
	} else {
		BuildQueueReference = nil
	}

	variables, err := expandVariables(d)
	if err != nil {
		return nil, "", fmt.Errorf("Error expanding varibles: %+v", err)
	}

	BuildQueue := build.BuildQueue{
		Id:       BuildQueueReference,
		Name:     converter.String(d.Get("name").(string)),
		Path:     converter.String(d.Get("path").(string)),
		Revision: converter.Int(d.Get("revision").(int)),
		Repository: &build.BuildRepository{
			Url:           &repoURL,
			Id:            &repoID,
			Name:          &repoID,
			DefaultBranch: converter.String(repository["branch_name"].(string)),
			Type:          converter.String(string(repoType)),
			Properties: &map[string]string{
				"connectedServiceId": repository["service_connection_id"].(string),
				"apiUrl":             repoAPIURL,
				"reportBuildStatus":  strconv.FormatBool(repository["report_build_status"].(bool)),
			},
		},
		Process: &build.YamlProcess{
			YamlFilename: converter.String(repository["yml_path"].(string)),
		},
		QueueStatus:    &build.DefinitionQueueStatusValues.Enabled,
		Type:           &build.DefinitionTypeValues.Build,
		Quality:        &build.DefinitionQualityValues.Definition,
		VariableGroups: expandVariableGroups(d),
		Variables:      variables,
		Triggers:       &buildTriggers,
	}

	if agentPoolName, ok := d.GetOk("agent_pool_name"); ok {
		BuildQueue.Queue = &build.AgentPoolQueue{
			Name: converter.StringFromInterface(agentPoolName),
			Pool: &build.TaskAgentPoolReference{
				Name: converter.StringFromInterface(agentPoolName),
			},
		}
	}

	return &BuildQueue, projectID, nil
}

/**
 * certain types of build definitions require a service connection to run. This function
 * returns an error if a service connection was needed but not provided
 */
func validateServiceConnectionIDExistsIfNeeded(d *schema.ResourceData) error {
	repositories := d.Get("repository").([]interface{})
	repository := repositories[0].(map[string]interface{})

	repoType := repository["repo_type"].(string)
	serviceConnectionID := repository["service_connection_id"].(string)

	if strings.EqualFold(repoType, string(model.RepoTypeValues.Bitbucket)) && serviceConnectionID == "" {
		return errors.New("bitbucket repositories need a referenced service connection ID")
	}
	if strings.EqualFold(repoType, string(model.RepoTypeValues.GitHubEnterprise)) && serviceConnectionID == "" {
		return errors.New("GitHub Enterprise repositories need a referenced service connection ID")
	}
	return nil
}

func buildVariableGroup(id int) *build.VariableGroup {
	return &build.VariableGroup{
		Id: &id,
	}
}
