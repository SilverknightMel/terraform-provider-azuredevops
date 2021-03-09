package pipelines

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/pipelines"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/client"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/utils/converter"
)

const (
	rpVariable              = "variable"
	rpVariableName          = "name"
	rpVariableValue         = "value"
	rpSecretVariableValue   = "secret_value"
	rpVariableIsSecret      = "is_secret"
)

// ResourceRunPipeline schema and implementation for queue build resource

func ResourceRunPipeline() *schema.Resource {
	return &schema.Resource{
		Create: resourceAzureRunPipelineCreate,
		Read:   resourceAzureRunPipelineRead,
		Update: resourceAzureRunPipelineUpdate,
		Delete: resourceAzureRunPipelineDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"build_id": {
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
			"parameter_terraformIntent": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_customerServiceConnectionName": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_projectOwner": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_artifactoryDir": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_artifactoryKey": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_operateServerNames": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_serverOperation": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_rgName": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_accessKey": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_ARM_CLIENT_ID": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_clientSec": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_Maintenanceminutes": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"parameter_comment": {
				Type:         schema.TypeString,
				ForceNew:     false,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},

		},
	}
}
func resourceAzureRunPipelineCreate(d *schema.ResourceData, m interface{}) error {
	clients := m.(*client.AggregatedClient)
 	RunPipeline, err := expandRunPipeline(d, true)
	projectName := converter.String(d.Get("project_name").(string)),
	if err != nil {
		return fmt.Errorf("Error converting terraform data model to AzDO RunPipeline reference: %+v", err)
	}

	createdRunPipeline, err := createAzureRunPipeline(clients, RunPipeline, projectName)
	if err != nil {
		return fmt.Errorf("Error creating agent pool in Azure DevOps: %+v", err)
	}


	return resourceAzureRunPipelineRead(d, m)
}

func createAzureRunPipeline(clients *client.AggregatedClient, RunPipeline *pipelines.RunPipelineParameters, project string ) (*pipelines.run, error) {
	createRunPipeline, err := clients.BuildClient.
	
	args := taskagent.AddRunPipelineArgs{
		Pool: RunPipeline,
	}

	newTaskAgent, err := clients.TaskAgentClient.AddRunPipeline(clients.Ctx, args)
	return newTaskAgent, err
}


func expandRunPipeline(d *schema.ResourceData, forCreate bool) (*pipelines.RunPipelineParameters  , error) {

	variables, err := expandVariables(d)
	if err != nil {
		return "", fmt.Errorf("Error expanding varibles: %+v", err)
	}
	RunPipeline := pipelines.RunPipelineParameters{
		Resources: &pipelines.RunResourcesParameters{
			Repositories: &pipelines.RepositoryResourceParameters{
			RefName: converter.String(d.Get("sourceBranch").(string)),
			Token: converter.String(d.Get("parameter_accessKey").(string)),
			},
		},
		Variables: variables,
	}



	return &RunPipeline,nil
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
			IsSecret:      isSecret,
			Value:         val,
		}
	}

	return &expandedVars, nil
}
