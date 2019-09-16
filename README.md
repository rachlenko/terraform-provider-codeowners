# CODEOWNERS Terraform Provider

[![Build Status](https://travis-ci.com/form3tech-oss/terraform-provider-codeowners.svg?branch=master)](https://travis-ci.com/form3tech-oss/terraform-provider-codeowners)

Terraform Provider for GitHub [CODEOWNERS](https://help.github.com/articles/about-code-owners/) files.

## Summary

Do you use terraform to manage your GitHub organisation? Are you frustrated that you can't manage your code review approvers using the same method? Well, now you can!

## Installation

Download the relevant binary from [releases](https://github.com/form3tech-oss/terraform-provider-codeowners/releases) and copy it to `$HOME/.terraform.d/plugins/`.

## Configuration

The following provider block variables are available for configuration:

- `commit_message_prefix` - An optional prefix to be added to all commits generated as a result of manipulating the `CODEOWNERS` file.
- `github_token` GitHub auth token - see below section. (read from env var `$GITHUB_TOKEN`)
- `username` Username to use in commits (read from env var `$GITHUB_USERNAME`)
- `email` Email to use in commits - this must match the email in your GPG key if you are signing commits (read from env var `$GITHUB_EMAIL`)
- `gpg_secret_key` The private GPG key to use to sign commits (optional) (read from env var `$GPG_SECRET_KEY`)
- `gpg_passphrase` The passphrase associated with the aforementioned GPG key (optional) (read from env var `$GPG_PASSPHRASE`)

### Authentication

There are two methods for authenticating with this provider.

You can specify your github token in the `provider` block, as below:

```hcl
provider "codeowners" {
    github_token = "..."
}
```

Alternatively, you can use the following environment variable:

```bash
export GITHUB_TOKEN="..."
```

Provider block variables will override environment variables, where provided.

Your token must have the full `repo` permission block set.

## Resources

### `codeowners_file`

```hcl
resource "codeowners_file" "my-codeowners-file" {
  # for repo github.com/my-org/my-repo
  repository_name  = "my-repo"
  repository_owner = "my-org"
  branch           = "master" # this is where changes will be committed - you can omit this to use the default repo branch (recommended)
  rules = [
    {
      pattern = "*"
      usernames = [ "expert" ]
    },
    {
      pattern = "*.java"
      usernames = [ "java-expert", "my-org/experts" ]
    }
  ]
}
```

The above would result in the following content being committed to `.github/CODEOWNERS` on `master` of the `github.com/my-org/my-repo` repository:

```
# automatically generated by terraform - please do not edit here
* @expert 
*.java @java-expert @my-org/experts
```
