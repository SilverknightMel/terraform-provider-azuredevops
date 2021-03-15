# Make sure to set the following environment variables:
#   AZDO_PERSONAL_ACCESS_TOKEN
#   AZDO_ORG_SERVICE_URL
terraform {
  required_providers {
    azuredevops = {
      source = "microsoft/azuredevops"
      version = ">=0.1.0"
    }
  }
}


// This section creates a project
data "azuredevops_project" "project" {
  name       = "PlatformServicesOperations"


}

resource "azuredevops_pipelines_run" "runPipeline" {

}

// This section configures variable groups and a build definition
/* resource "azuredevops_build_definition" "build" {
  project_id = data.azuredevops_project.project.id
  name       = "Sample Build Definition"
  path       = "\\ExampleFolder"

  repository {
    repo_type   = "TfsGit"
    repo_id     = azuredevops_git_repository.repository.id
    branch_name = azuredevops_git_repository.repository.default_branch
    yml_path    = "azure-pipelines.yml"
  }

  variable_groups = [azuredevops_variable_group.vg.id]
} */

// This section configures an Azure DevOps Variable Group
resource "azuredevops_variable_group" "vg" {
  project_id   = data.azuredevops_project.project.id
  name         = "Sample VG 1"
  description  = "A sample variable group."
  allow_access = true

  variable {
    name      = "key1"
    value     = "value1"
    is_secret = true
  }

  variable {
    name  = "key2"
    value = "value2"
  }

  variable {
    name = "key3"
  }
}

// This section configures an Azure DevOps Git Repository with branch policies
/* resource "azuredevops_git_repository" "repository" {
  project_id = data.azuredevops_project.project.id
  name       = "Sample Repo"
  initialization {
    init_type = "Clean"
  }
} */


 output "azuredevops_project" {
  value = data.azuredevops_project.project.id
}
