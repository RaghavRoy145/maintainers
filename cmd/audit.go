/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"maintainers/pkg/utils"
)

// auditCmd represents the audit command
var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "ensure OWNERS, OWNERS_ALIASES and sigs.yaml have the correct data structure",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Running script : %s\n", time.Now().Format("01-02-2006 15:04:05"))
		pwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		sigsYamlPath, err := utils.GetSigsYamlFile(pwd)
		if err != nil {
			panic(fmt.Errorf("unable to find sigs.yaml file: %w", err))
		}
		context, err := utils.GetSigsYaml(sigsYamlPath)
		if err != nil {
			panic(fmt.Errorf("error parsing file: %s - %w", sigsYamlPath, err))
		}

		groupMap := map[string][]utils.Group{
			"sigs":          (*context).Sigs,
			"usergroups":    (*context).UserGroups,
			"workinggroups": (*context).WorkingGroups,
			"committees":    (*context).Committees,
		}

		for groupType, groups := range groupMap {
			for _, group := range groups {
				auditGroup(pwd, groupType, group, context)
			}
		}
		fmt.Printf("Done.\n")
	},
}

func auditGroup(pwd string, groupType string, group utils.Group, context *utils.Context) {
	if len(group.Dir) == 0 {
		fmt.Printf("WARNING: missing 'dir' for a group under %s/%s\n", groupType, group.Name)
	}
	if len(group.Name) == 0 {
		fmt.Printf("WARNING: missing 'name' for a group under %s/%s\n", groupType, group.Dir)
	}
	fmt.Printf(">>>> Processing %s [%s/%s]\n", groupType, group.Dir, group.Name)
	if len(group.MissionStatement) == 0 {
		fmt.Printf("WARNING: missing 'mission_statement' for %s/%s\n", groupType, group.Dir)
	}
	if len(group.CharterLink) == 0 {
		fmt.Printf("WARNING: missing 'charter_link' for %s/%s\n", groupType, group.Dir)
	} else {
		auditCharterLink(pwd, groupType, group)
	}
	if groupType == "workinggroups" {
		auditWorkingGroupStakeholders(group, groupType, context)
	}
	if len(group.Label) == 0 {
		fmt.Printf("WARNING: missing 'label' for %s/%s\n", groupType, group.Dir)
	}
	auditLeadership(group, groupType)
	if len(group.Meetings) == 0 {
		fmt.Printf("WARNING: missing 'meetings' for %s/%s\n", groupType, group.Dir)
	}
	auditContact(&group.Contact, groupType, group)
	if len(group.Subprojects) == 0 {
		fmt.Printf("WARNING: missing 'subprojects' for a group under %s/%s\n", groupType, group.Dir)
	} else {
		auditSubProject(group, groupType)
	}
}

func auditSubProject(group utils.Group, groupType string) {
	for _, subproject := range group.Subprojects {
		extra := fmt.Sprintf("[%s/%s]", subproject.Name, subproject.Description)
		if len(subproject.Name) == 0 {
			fmt.Printf("WARNING: missing name for subproject %v for a group under %s/%s\n", extra, groupType, group.Dir)
		}
		if len(subproject.Description) == 0 {
			fmt.Printf("WARNING: missing description for subproject %v for a group under %s/%s\n", extra, groupType, group.Dir)
		}
		if subproject.Contact == nil {
			fmt.Printf("WARNING: missing contact for subproject %v for a group under %s/%s\n", extra, groupType, group.Dir)
		} else {
			auditContact(subproject.Contact, groupType, group)
		}
		if len(subproject.Owners) == 0 {
			fmt.Printf("WARNING: missing owners for subproject %v for a group under %s/%s\n", extra, groupType, group.Dir)
		}
		if len(subproject.Meetings) == 0 {
			fmt.Printf("WARNING: missing meetings for subproject %v for a group under %s/%s\n", extra, groupType, group.Dir)
		}
	}
}

func auditContact(contact *utils.Contact, groupType string, group utils.Group) {
	if len(contact.Slack) == 0 {
		fmt.Printf("WARNING: missing 'slack' for %s/%s under contact\n", groupType, group.Dir)
	}
	if len(contact.MailingList) == 0 {
		fmt.Printf("WARNING: missing 'mailing_list' for %s/%s under contact\n", groupType, group.Dir)
	}
	if len(contact.PrivateMailingList) == 0 {
		fmt.Printf("WARNING: missing 'private_mailing_list' for %s/%s under contact\n", groupType, group.Dir)
	}
	if len(contact.GithubTeams) == 0 {
		fmt.Printf("WARNING: missing 'teams' for %s/%s under contact\n", groupType, group.Dir)
	}
	if contact.Liaison != nil {
		auditPerson(group, groupType, "contact/liaison", contact.Liaison)
	}
}

func auditCharterLink(pwd string, groupType string, group utils.Group) {
	if strings.HasPrefix(group.CharterLink, "http") {
		client := &http.Client{}
		resp, err := client.Get(group.CharterLink)
		if err != nil || resp.StatusCode != http.StatusOK {
			fmt.Printf("WARNING: unable to reach url %s for %s/%s\n", group.CharterLink, groupType, group.Dir)
		}
	} else {
		charterPath := path.Join(pwd, group.Dir, group.CharterLink)
		if _, err := os.Stat(charterPath); errors.Is(err, os.ErrNotExist) {
			fmt.Printf("WARNING: missing file %s for %s/%s\n", charterPath, groupType, group.Dir)
		}
	}
}

func auditWorkingGroupStakeholders(group utils.Group, groupType string, context *utils.Context) {
	if len(group.StakeholderSIGs) == 0 {
		fmt.Printf("WARNING: missing 'stakeholder_sigs' for %s/%s\n", groupType, group.Dir)
	} else {
		for _, stakeholder := range group.StakeholderSIGs {
			found := false
			for _, group := range context.Sigs {
				if group.Name == stakeholder {
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("WARNING: stakeholder_sigs entry '%s' not found for %s/%s\n", stakeholder, groupType, group.Dir)
			}
		}
	}
}

func auditLeadership(group utils.Group, groupType string) {
	if len(group.Leadership.Chairs) == 0 {
		fmt.Printf("WARNING: missing 'leadership/chairs' for %s/%s\n", groupType, group.Dir)
		if groupType == "sigs" {
			if len(group.Leadership.Chairs) == 1 {
				fmt.Printf("WARNING: please consider adding more folks in 'leadership/chairs' for %s/%s\n", groupType, group.Dir)
			}
		}
	}
	if len(group.Leadership.TechnicalLeads) == 0 {
		fmt.Printf("WARNING: missing 'leadership/tech_leads' for %s/%s\n", groupType, group.Dir)
		if groupType == "sigs" {
			fmt.Printf("WARNING: if chairs are serving as tech leads, please add them explicitly in 'leadership/tech_leads' for %s/%s\n", groupType, group.Dir)
		}
	}
	var persons []utils.Person
	persons = append(persons, group.Leadership.Chairs...)
	persons = append(persons, group.Leadership.TechnicalLeads...)
	persons = append(persons, group.Leadership.EmeritusLeads...)
	for _, person := range persons {
		auditPerson(group, groupType, "leadership", &person)
	}
}

func auditPerson(group utils.Group, groupType string, extra string, person *utils.Person) {
	if len(person.Name) == 0 {
		fmt.Printf("WARNING: missing %s name for %v for %s/%s\n", extra, person, groupType, group.Dir)
	}
	if len(person.GitHub) == 0 {
		fmt.Printf("WARNING: missing %s github id for %v for %s/%s\n", extra, person, groupType, group.Dir)
	}
	if len(person.Company) == 0 {
		fmt.Printf("WARNING: missing %s company for %v for %s/%s\n", extra, person, groupType, group.Dir)
	}
}

func init() {
	rootCmd.AddCommand(auditCmd)
}