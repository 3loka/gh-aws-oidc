# gh-aws-oidc
This is a Github CLI extension to setup a federated OIDC based connection between a repository, organization or an environment and AWS Services. 

## Usage instructions
Note: Before you run the CLI, run [aws configure](https://docs.aws.amazon.com/cli/latest/reference/configure/). You will need the Access Key combinations to connect initially to AWS. aws configure will create a config file under ~/.aws/config. Once you complete this CLI, you can remove the AWS credentials by deleting the folder
```
gh aws-oidc [flags]

    Options
        --o <organization>
        Select a organization to connect to AWS
        --e <environment>
        Select an environment under the repository 
        --useDefaults
        This will skip the interactive flow and select the defaults on aws side to setup connection quickly (Not implemented yet, work in progress)

    Options inherited from parent commands
        -R, --repo <[HOST/]OWNER/REPO>
        Select another repository using the [HOST/]OWNER/REPO format


Examples
# Setup connection at a repo level (assuming user is in the git folder)
$ gh aws-oidc

# Setup connection at an organization level
$ gh aws-oidc -o myorg

# Setup connection at a repo level
$  gh aws-oidc -R myorg/myrepo

# Setup connection at a repo and environment level
$  gh aws-oidc -R myorg/myrepo -e myenvironment

```

![Demo](https://github.com/3loka/gh-aws-oidc/blob/main/demo.gif)

# To run this application, execute below command
- Install the CLI extension - Follow the steps here https://github.com/cli/cli#installation
- Clone this repo - `git clone https://github.com/3loka/gh-aws-oidc.git`
- Build the code - `go build`


