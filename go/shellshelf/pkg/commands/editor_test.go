package commands

import (
	"fmt"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"reflect"
	"testing"
)

func TestGetParsedCommand(t *testing.T) {
	tests := []struct {
		Name     string
		Input    string
		Expected models.Command
	}{
		{
			Name:  "empty input",
			Input: "",
			Expected: models.Command{
				Name:        "",
				Description: "",
				Tags:        []string{},
				Command:     "",
			},
		},
		{
			Name: "can parse simplest, complete command",
			Input: `+~~~~+~~~~+ name:
Name
+~~~~+~~~~+ description:
Description
+~~~~+~~~~+ tags:
Tag1
+~~~~+~~~~+ command:
echo "world"
+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "Description",
				Tags: []string{
					"Tag1",
				},
				Command: "echo \"world\"",
			},
		},
		{
			Name: "can parse empty fields",
			Input: `+~~~~+~~~~+ name:

+~~~~+~~~~+ description:

+~~~~+~~~~+ tags:

+~~~~+~~~~+ command:

+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "",
				Description: "",
				Tags:        []string{},
				Command:     "",
			},
		},
		{
			Name: "can parse mix of empty/filled fields",
			Input: `+~~~~+~~~~+ name:
Name
+~~~~+~~~~+ description:

+~~~~+~~~~+ tags:
tag1
+~~~~+~~~~+ command:
command stuff
+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "",
				Tags: []string{
					"tag1",
				},
				Command: "command stuff",
			},
		},
		{
			Name: "can parse multiple lines of tags, one by line",
			Input: `+~~~~+~~~~+ name:
Name
+~~~~+~~~~+ description:
Description
+~~~~+~~~~+ tags:
tag1
tag2
+~~~~+~~~~+ command:
command stuff
+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "Description",
				Tags: []string{
					"tag1",
					"tag2",
				},
				Command: "command stuff",
			},
		},
		{
			Name: "can parse command with multiple lines",
			Input: `+~~~~+~~~~+ name:
Name
+~~~~+~~~~+ description:
Description
Line2

Line3

+~~~~+~~~~+ tags:

tag1

tag2

+~~~~+~~~~+ command:

command stuff
Line2
Line3

+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "Description\nLine2\n\nLine3",
				Tags: []string{
					"tag1",
					"tag2",
				},
				Command: "command stuff\nLine2\nLine3",
			},
		},
		{
			Name: "can parse multiple lines of tags, also with comma-separated tags",
			Input: `+~~~~+~~~~+ name:
Name
+~~~~+~~~~+ description:
Description
+~~~~+~~~~+ tags:
tag1, tag2
, tag3, tag4,
tag5

tag6
+~~~~+~~~~+ command:
command stuff
+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "Description",
				Tags: []string{
					"tag1",
					"tag2",
					"tag3",
					"tag4",
					"tag5",
					"tag6",
				},
				Command: "command stuff",
			},
		},
		{
			Name: "can parse command with contiguous missing fields",
			Input: `+~~~~+~~~~+ name:
Name
+~~~~+~~~~+ description:
+~~~~+~~~~+ tags:
+~~~~+~~~~+ command:
command stuff
+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "",
				Tags:        []string{},
				Command:     "command stuff",
			},
		},
		{
			Name: "can parse command with non-contiguous missing fields",
			Input: `+~~~~+~~~~+ name:
Name
+~~~~+~~~~+ description:
+~~~~+~~~~+ tags:
tag1

+~~~~+~~~~+ command:
command stuff
+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "",
				Tags: []string{
					"tag1",
				},
				Command: "command stuff",
			},
		},
		{
			Name: "can parse command with missing fields and multiple lines",
			Input: `+~~~~+~~~~+ name:

Name

+~~~~+~~~~+ description:



+~~~~+~~~~+ tags:


tag1

+~~~~+~~~~+ command:
command stuff
+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "",
				Tags: []string{
					"tag1",
				},
				Command: "command stuff",
			},
		},
		{
			Name: "can parse command with escape sequences",
			Input: `+~~~~+~~~~+ name:
Name
+~~~~+~~~~+ description:
+~~~~+~~~~+ tags:
+~~~~+~~~~+ command:
command stuff
+~~~~+~~~~+`,
			Expected: models.Command{
				Name:        "Name",
				Description: "",
				Tags:        []string{},
				Command:     "command stuff",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			o, _ := GetParsedCommand(test.Input)
			if !reflect.DeepEqual(o, test.Expected) {
				fmt.Println("o:", o)
				res := fmt.Sprintf("name='%s', description='%s', tags=%v, command='%s'",
					test.Expected.Name, test.Expected.Description, test.Expected.Tags, test.Expected.Command)
				exp := fmt.Sprintf("name='%s', description='%s', tags=%v, command='%s'",
					o.Name, o.Description, o.Tags, o.Command)
				t.Errorf("\nExpected: %v\nGot:      %v", res, exp)
			}
		})
	}
}
