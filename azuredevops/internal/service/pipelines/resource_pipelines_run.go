package pipelines

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/pipelines"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/client"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/utils"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/utils/converter"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/utils/tfhelper"
)

const (
	rpVariable            = "variable"
	rpVariableName        = "name"
	rpVariableValue       = "value"
	rpSecretVariableValue = "secret_value"
	rpVariableIsSecret    = "is_secret"
)

// ResourceRunPipeline schema and implementation for queue build resource

func ResourceRunPipeline() *schema.Resource {
	return &schema.Resource{
		Create: resourceAzureRunPipelineCreate,
		Read:   resourceAzureRunPipelineRead,
		Update: resourceAzureRunPipelineRead,
		Delete: resourceAzureRunPipelineRead,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"pipeline_id": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"project_name": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"sourceBranch": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			rpVariable: {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						rpVariableName: {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotWhiteSpace,
						},
						rpVariableValue: {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						rpSecretVariableValue: {
							Type:      schema.TypeString,
							Optional:  true,
							Sensitive: true,
							Default:   "",
						},
						rpVariableIsSecret: {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
					},
				},
			},
		},
	}
}
func resourceAzureRunPipelineCreate(d *schema.ResourceData, m interface{}) error {
	clients := m.(*client.AggregatedClient)
	RunPipeline, projectName, PipelineId, err := expandRunPipeline(d, true)

	if err != nil {
		return fmt.Errorf("Error converting terraform data model to AzDO RunPipeline reference: %+v", err)
	}

	createdRunPipeline, err := createAzureRunPipeline(clients, RunPipeline, projectName, PipelineId)
	if err != nil {
		return fmt.Errorf("Error creating agent pool in Azure DevOps: %+v", err)
	}

	//flattenRunPipeline(d, createdRunPipeline, projectName)

	err = waitForRunPipeline(clients, createdRunPipeline, projectName, PipelineId)
	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(*createdRunPipeline.Id))
	return resourceAzureRunPipelineRead(d, m)
}

func waitForRunPipeline(clients *client.AggregatedClient, createdRunPipeline *pipelines.Run, project_Name string, Pipeline_Id int) error {
	runID := createdRunPipeline.Id
	stateConf := &resource.StateChangeConf{
		Pending: []string{"InProgress", "Canceling", "Unknown"},
		Target:  []string{"Completed"},
		Refresh: func() (interface{}, string, error) {
			state := "InProgress"
			pipelineStatus, err := pipelineStatusRead(clients, project_Name, Pipeline_Id, *runID)
			if err != nil {
				return nil, "", fmt.Errorf("Error reading repository: %+v", err)
			}

			if converter.ToString((*string)(pipelineStatus.State), "") == "Completed" {
				state = "Completed"
			}

			return state, state, nil
		},
		Timeout:                   60 * time.Second,
		MinTimeout:                2 * time.Second,
		Delay:                     1 * time.Second,
		ContinuousTargetOccurence: 1,
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("Error retrieving expected branch for repository [%s]: %+v", err)
	}
	return nil
}

func pipelineStatusRead(clients *client.AggregatedClient, project_Name string, Pipeline_Id int, run_ID int) (*pipelines.Run, error) {
	getRunArgs := pipelines.GetRunArgs{
		Project:    &project_Name,
		PipelineId: &Pipeline_Id,
	}

	runStatus, err := clients.PipelineClient.GetRun(clients.Ctx, getRunArgs)

	if err != nil {
		return nil, fmt.Errorf("Failed to locate parent repository [%s]: %+v")
	}

	return runStatus, err
}

func createAzureRunPipeline(clients *client.AggregatedClient, RunPipeline *pipelines.RunPipelineParameters, projectName string, Pipeline_Id int) (*pipelines.Run, error) {
	createRunPipeline, err := clients.PipelineClient.RunPipeline(clients.Ctx, pipelines.RunPipelineArgs{
		RunParameters: RunPipeline,
		Project:       &projectName,
		PipelineId:    &Pipeline_Id,
	})

	return createRunPipeline, err
}

func resourceAzureRunPipelineRead(d *schema.ResourceData, m interface{}) error {
	clients := m.(*client.AggregatedClient)
	Project_name := d.Get("project_name").(string)
	Pipeline_Id := d.Get("pipeline_id").(int)

	getPipelineArgs := pipelines.GetPipelineArgs{
		Project:    &Project_name,
		PipelineId: &Pipeline_Id,
	}

	//projectID, runPipelinesID, err := tfhelper.ParseProjectIDAndResourceID(d)

	RunPipeline, err := clients.PipelineClient.GetPipeline(clients.Ctx, getPipelineArgs)

	if err != nil {
		if utils.ResponseWasNotFound(err) {
			d.SetId("")
			return nil
		}
		return err
	}
	d.Set("name", RunPipeline.Name)
	d.Set("Folder ", RunPipeline.Folder)
	d.Set("Id", RunPipeline.Id)
	d.Set("Url", RunPipeline.Url)

	//flattenRunPipeline(d, RunPipeline, projectID)
	return nil
}

/* func flattenRunPipeline(d *schema.ResourceData, buildDefinition *pipelines.RunPipelineArgs, projectID string) {
	d.SetId(strconv.Itoa(*buildDefinition.Id))

	d.Set("project_id", projectID)
	d.Set("name", *buildDefinition.Name)
	d.Set("path", *buildDefinition.Path)
	//d.Set("repository", flattenRepository(buildDefinition))

	if buildDefinition.Queue != nil && buildDefinition.Queue.Pool != nil {
		d.Set("agent_pool_name", *buildDefinition.Queue.Pool.Name)
	}

	//d.Set("variable_groups", flattenVariableGroups(buildDefinition))
	d.Set(rpVariable, flattenRunPipelineVariables(d, buildDefinition))

	if buildDefinition.Triggers != nil {
		yamlCiTrigger := hasSettingsSourceType(buildDefinition.Triggers, build.DefinitionTriggerTypeValues.ContinuousIntegration, 2)
		d.Set("ci_trigger", flattenBuildDefinitionTriggers(buildDefinition.Triggers, yamlCiTrigger, build.DefinitionTriggerTypeValues.ContinuousIntegration))

		yamlPrTrigger := hasSettingsSourceType(buildDefinition.Triggers, build.DefinitionTriggerTypeValues.PullRequest, 2)
		d.Set("pull_request_trigger", flattenBuildDefinitionTriggers(buildDefinition.Triggers, yamlPrTrigger, build.DefinitionTriggerTypeValues.PullRequest))
	}

	revision := 0
	if buildDefinition.Revision != nil {
		revision = *buildDefinition.Revision
	}

	d.Set("revision", revision)
} */

func flattenRunPipelineVariables(d *schema.ResourceData, buildDefinition *build.BuildDefinition) interface{} {
	if buildDefinition.Variables == nil {
		return nil
	}
	variables := make([]map[string]interface{}, len(*buildDefinition.Variables))

	index := 0
	for varName, varVal := range *buildDefinition.Variables {
		var variable map[string]interface{}

		isSecret := converter.ToBool(varVal.IsSecret, false)
		variable = map[string]interface{}{
			rpVariableName:     varName,
			rpVariableValue:    converter.ToString(varVal.Value, ""),
			rpVariableIsSecret: isSecret,
		}

		//read secret variable from state if exist
		if isSecret {
			if stateVal := tfhelper.FindMapInSetWithGivenKeyValue(d, rpVariable, rpVariableName, varName); stateVal != nil {
				variable = stateVal
			}
		}
		variables[index] = variable
		index = index + 1
	}

	return variables
}

func expandRunPipeline(d *schema.ResourceData, forCreate bool) (*pipelines.RunPipelineParameters, string, int, error) {
	Project := d.Get("project_name").(string)
	variables, err := expandVariables(d)
	mapkey := "testonly"
	Ref_Name := converter.String(d.Get("sourceBranch").(string))
	token := converter.String(d.Get("parameter_accessKey").(string))
	PipelineId := d.Get("pipeline_id").(int)
	if err != nil {
		return nil, "", 0, fmt.Errorf("Error expanding varibles: %+v", err)
	}
	Repository_resource := map[string]pipelines.RepositoryResourceParameters{
		mapkey: {
			RefName: Ref_Name,
			Token:   token,
		},
	}
	RunResource := pipelines.RunResourcesParameters{
		Repositories: &Repository_resource,
	}

	RunPipeline := pipelines.RunPipelineParameters{
		Resources: &RunResource,

		Variables: variables,
	}

	return &RunPipeline, Project, PipelineId, nil
}

func expandVariables(d *schema.ResourceData) (*map[string]pipelines.Variable, error) {
	variables := d.Get(rpVariable)
	if variables == nil {
		return nil, nil
	}

	variablesList := variables.(*schema.Set).List()
	if len(variablesList) == 0 {
		return nil, nil
	}

	expandedVars := map[string]pipelines.Variable{}
	for _, variable := range variablesList {
		varAsMap := variable.(map[string]interface{})
		varName := varAsMap[rpVariableName].(string)

		if _, ok := expandedVars[varName]; ok {
			return nil, fmt.Errorf("Unexpectedly found duplicate variable with name %s", varName)
		}

		isSecret := converter.Bool(varAsMap[rpVariableIsSecret].(bool))
		var val *string

		if *isSecret {
			val = converter.String(varAsMap[rpSecretVariableValue].(string))
		} else {
			val = converter.String(varAsMap[rpVariableValue].(string))
		}
		expandedVars[varName] = pipelines.Variable{
			IsSecret: isSecret,
			Value:    val,
		}
	}

	return &expandedVars, nil
}
