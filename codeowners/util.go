package codeowners

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/crypto/openpgp"
)

var codeownersPath = ".github/CODEOWNERS"

type File struct {
	RepositoryName  string
	RepositoryOwner string
	Branch          string
	Ruleset         Ruleset
}

type Ruleset []Rule

type Rule struct {
	Pattern   string
	Usernames []string
}

func (ruleset Ruleset) Equal(comparison Ruleset) bool {
	if len(ruleset) != len(comparison) {
		return false
	}
	for _, rule := range ruleset {
		found := false
		for _, comparisonRule := range comparison {
			if comparisonRule.Pattern == rule.Pattern {
				found = sameStringSlice(rule.Usernames, comparisonRule.Usernames)
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sameStringSlice(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	// create a map of string -> int
	diff := make(map[string]int, len(x))
	for _, _x := range x {
		// 0 value for int is 0, so just increment a counter for the string
		diff[_x]++
	}
	for _, _y := range y {
		// If the string _y is not in diff bail out early
		if _, ok := diff[_y]; !ok {
			return false
		}
		diff[_y]--
		if diff[_y] == 0 {
			delete(diff, _y)
		}
	}
	if len(diff) == 0 {
		return true
	}
	return false
}

func (ruleset Ruleset) Compile() []byte {
	if ruleset == nil {
		return []byte{}
	}
	output := "# automatically generated by terraform - please do not edit here\n"
	for _, rule := range ruleset {
		usernames := ""
		for _, username := range rule.Usernames {
			if !strings.Contains(username, "@") {
				usernames = fmt.Sprintf("%s@%s ", usernames, username)
			} else {
				usernames = fmt.Sprintf("%s%s ", usernames, username)
			}
		}
		output = fmt.Sprintf("%s%s %s\n", output, rule.Pattern, usernames)
	}
	return []byte(output)
}

func parseRulesFile(data string) Ruleset {

	rules := []Rule{}
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if trimmed[0] == '#' { // ignore comments
			continue
		}
		words := strings.Split(trimmed, " ")
		if len(words) < 2 {
			continue
		}
		rule := Rule{
			Pattern: words[0],
		}
		for _, username := range words[1:] {
			if len(username) == 0 { // may be split by multiple spaces
				continue
			}
			if username[0] == '@' {
				username = username[1:]
			}
			rule.Usernames = append(rule.Usernames, username)
		}
		rules = append(rules, rule)
	}

	return rules

}

type signedCommitOptions struct {
	repoOwner     string
	repoName      string
	commitMessage string
	gpgPassphrase string
	gpgPrivateKey string // detached armor format
	changes       []github.TreeEntry
	branch        string
	username      string
	email         string
}

func createCommit(client *github.Client, options *signedCommitOptions) error {
	ctx := context.Background()

	// get ref for selected branch
	ref, _, err := client.Git.GetRef(ctx, options.repoOwner, options.repoName, "refs/heads/"+options.branch)
	if err != nil {
		return err
	}

	// create tree containing required changes
	tree, _, err := client.Git.CreateTree(ctx, options.repoOwner, options.repoName, *ref.Object.SHA, options.changes)
	if err != nil {
		return err
	}

	// get parent commit
	parent, _, err := client.Repositories.GetCommit(ctx, options.repoOwner, options.repoName, *ref.Object.SHA)
	if err != nil {
		return err
	}

	// This is not always populated, but is needed.
	parent.Commit.SHA = github.String(parent.GetSHA())

	date := time.Now()
	author := &github.CommitAuthor{
		Date:  &date,
		Name:  github.String(options.username),
		Email: github.String(options.email),
	}

	var verification *github.SignatureVerification

	if options.gpgPrivateKey != "" {
		// the payload must be "an over the string commit as it would be written to the object database"
		// we sign this data to verify the commit
		payload := fmt.Sprintf(
			`tree %s
parent %s
author %s <%s> %d +0000
committer %s <%s> %d +0000

%s`,
			tree.GetSHA(),
			parent.GetSHA(),
			author.GetName(),
			author.GetEmail(),
			date.Unix(),
			author.GetName(),
			author.GetEmail(),
			date.Unix(),
			options.commitMessage,
		)

		// sign the payload data
		signature, err := signData(payload, options.gpgPrivateKey, options.gpgPassphrase)
		if err != nil {
			return err
		}

		verification = &github.SignatureVerification{
			Signature: signature,
		}
	}

	commit := &github.Commit{
		Author:       author,
		Message:      &options.commitMessage,
		Tree:         tree,
		Parents:      []github.Commit{*parent.Commit},
		Verification: verification,
	}
	newCommit, _, err := client.Git.CreateCommit(ctx, options.repoOwner, options.repoName, commit)
	if err != nil {
		return err
	}

	// Attach the commit to the selected branch
	ref.Object.SHA = newCommit.SHA
	_, _, err = client.Git.UpdateRef(ctx, options.repoOwner, options.repoName, ref, false)
	return err
}

func signData(data string, privateKey string, passphrase string) (*string, error) {

	entitylist, err := openpgp.ReadArmoredKeyRing(strings.NewReader(privateKey))
	if err != nil {
		return nil, err
	}
	pk := entitylist[0]

	ppb := []byte(passphrase)

	if pk.PrivateKey != nil && pk.PrivateKey.Encrypted {
		err := pk.PrivateKey.Decrypt(ppb)
		if err != nil {
			return nil, err
		}
	}

	for _, subkey := range pk.Subkeys {
		if subkey.PrivateKey != nil && subkey.PrivateKey.Encrypted {
			err := subkey.PrivateKey.Decrypt(ppb)
			if err != nil {
				return nil, err
			}
		}
	}

	out := new(bytes.Buffer)
	reader := strings.NewReader(data)
	if err := openpgp.ArmoredDetachSign(out, pk, reader, nil); err != nil {
		return nil, err
	}
	signature := string(out.Bytes())
	return &signature, nil
}
