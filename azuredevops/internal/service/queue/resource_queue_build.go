package queue

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/client"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/utils/converter"
)

// ResourceQueueBuild schema and implementation for queue build resource

func ResourceQueueBuild() *schema.Resource {
	return &schema.Resource{
		Create: resourceAzureQueueBuildCreate,
		Read:   resourceAzureQueueBuildRead,
		Update: resourceAzureQueueBuildUpdate,
		Delete: resourceAzureQueueBuildDelete,
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
func resourceAzureQueueBuildCreate(d *schema.ResourceData, m interface{}) error {
	clients := m.(*client.AggregatedClient)
	QueueBuild, err := expandQueueBuild(d, true)
	if err != nil {
		return fmt.Errorf("Error converting terraform data model to AzDO QueueBuild reference: %+v", err)
	}

	createdQueueBuild, err := createAzureQueueBuild(clients, QueueBuild)
	if err != nil {
		return fmt.Errorf("Error creating agent pool in Azure DevOps: %+v", err)
	}


	return resourceAzureQueueBuildRead(d, m)
}

func createAzureQueueBuild(clients *client.AggregatedClient, QueueBuild *build.QueueBuildArgs ) (*build.BuildQueuedEvent, error) {
	args := taskagent.AddQueueBuildArgs{
		Pool: QueueBuild,
	}

	newTaskAgent, err := clients.TaskAgentClient.AddQueueBuild(clients.Ctx, args)
	return newTaskAgent, err
}


func expandQueueBuild(d *schema.ResourceData, forCreate bool) (*build.QueueBuildArgs  , error) {
	QueueBuild := build.QueueBuildArgs{

	Build: &build.Build {
		Id:         converter.Int(d.Get("build_id").(int)),
		SourceBranch:    converter.String(d.Get("sourceBranch").(string)),

	},
	Project : converter.String(d.Get("project_name").(string)),

	}


	return &QueueBuild,nil
}
