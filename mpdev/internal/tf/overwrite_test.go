// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package tf

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestOverwriteTf(t *testing.T) {

	testcases := []struct {
		name            string
		tfFiles         map[string]string
		expectedTfFiles map[string]string
		overwriteConfig overwriteConfig
		errorContains   string
	}{{
		name: "Overwrite multiple variables and files",
		tfFiles: map[string]string{
			"main.tf":        mainTf,
			"anyfilename.tf": otherTf,
		},
		expectedTfFiles: map[string]string{
			"main.tf":        mainTfReplaced,
			"anyfilename.tf": otherTfReplaced,
		},
		overwriteConfig: overwriteConfig{
			Variables: []string{"value_to_replace", "other_value_to_replace", "another_variable"},
			Replacements: map[string]string{
				"original-value": "new-value",
				"old-value":      "newer-value",
				"oldest-value":   "newest-value",
			},
		},
	}, {
		name: "Invalid HCL shows parsing error",
		tfFiles: map[string]string{
			"main.tf": "this is broken",
		},
		overwriteConfig: overwriteConfig{
			Variables: []string{"value_to_replace"},
			Replacements: map[string]string{
				"original-value": "new-value",
			},
		},
		errorContains: "failure parsing terraform module",
	}, {
		name: "Fail when variable not present in Terraform module",
		tfFiles: map[string]string{
			"main.tf": mainTf,
		},
		overwriteConfig: overwriteConfig{
			Variables: []string{"missing_variable"},
			Replacements: map[string]string{
				"original-value": "new-value",
			},
		},
		errorContains: "variable: missing_variable not found",
	}, {
		name: "Fail when variable has no default value set",
		tfFiles: map[string]string{
			"main.tf": tfNoDefault,
		},
		overwriteConfig: overwriteConfig{
			Variables: []string{"value_to_replace"},
			Replacements: map[string]string{
				"original-value": "new-value",
			},
		},
		errorContains: "variable: value_to_replace must have default value",
	}, {
		name: "Fail when variable default value is not a string",
		tfFiles: map[string]string{
			"main.tf": tfDefaultWrongType,
		},
		overwriteConfig: overwriteConfig{
			Variables: []string{"value_to_replace"},
			Replacements: map[string]string{
				"original-value": "new-value",
			},
		},
		errorContains: "image variable: value_to_replace must be type string",
	}, {
		name: "Fail when variable default value is not in replacements",
		tfFiles: map[string]string{
			"main.tf": mainTf,
		},
		overwriteConfig: overwriteConfig{
			Variables: []string{"value_to_replace"},
			Replacements: map[string]string{
				"non-existent": "new-value",
			},
		},
		errorContains: "default value: original-value of variable: value_to_replace not found in replacements",
	}, {
		name: "With NewValues, overwrite multiple variables and files",
		tfFiles: map[string]string{
			"main.tf":        mainTf,
			"anyfilename.tf": otherTf,
		},
		expectedTfFiles: map[string]string{
			"main.tf":        mainTfReplaced,
			"anyfilename.tf": otherTfReplaced,
		},
		overwriteConfig: overwriteConfig{
			NewValues: map[string]string{
				"value_to_replace":       "new-value",
				"other_value_to_replace": "newer-value",
				"another_variable":       "newest-value",
			},
		},
	}, {
		name: "With NewValues, overwrite multiple variables and files. Add consumer label",
		tfFiles: map[string]string{
			"main.tf": mainTfNoLabel,
		},
		expectedTfFiles: map[string]string{
			"main.tf": mainTfLabelUpserted,
		},
		overwriteConfig: overwriteConfig{
			ConsumerLabel: "new-consumer-label",
			NewValues: map[string]string{
				"value_to_replace": "new-value",
			},
		},
	}, {
		name: "With NewValues, overwrite multiple variables and files. Do not replace existing consumer label",
		tfFiles: map[string]string{
			"main.tf": mainTfProvidedLabel,
		},
		expectedTfFiles: map[string]string{
			"main.tf": mainTfLabelUpserted,
		},
		overwriteConfig: overwriteConfig{
			ConsumerLabel: "even-newer-consumer-label",
			NewValues: map[string]string{
				"value_to_replace": "new-value",
			},
		},
	},
		{
			name: "With NewValues, ignores Variables and Replacements",
			tfFiles: map[string]string{
				"main.tf":        mainTf,
				"anyfilename.tf": otherTf,
			},
			expectedTfFiles: map[string]string{
				"main.tf":        mainTfReplaced,
				"anyfilename.tf": otherTfReplaced,
			},
			overwriteConfig: overwriteConfig{
				NewValues: map[string]string{
					"value_to_replace":       "new-value",
					"other_value_to_replace": "newer-value",
					"another_variable":       "newest-value",
				},
				Variables: []string{"value_to_replace", "other_value_to_replace", "another_variable"},
				Replacements: map[string]string{
					"original-value": "new-value-unused",
					"old-value":      "newer-value-unused",
					"oldest-value":   "newest-value-unused",
				},
			},
		}, {
			name: "With NewValues, adds default value when variable has no default value set",
			tfFiles: map[string]string{
				"main.tf": tfNoDefault,
			},
			expectedTfFiles: map[string]string{
				"main.tf": tfDefaultAdded,
			},
			overwriteConfig: overwriteConfig{
				NewValues: map[string]string{
					"value_to_replace": "new-value",
				},
			},
		}, {
			name: "With NewValues, fail when variable default value is not a string",
			tfFiles: map[string]string{
				"main.tf": tfNoDefaultWrongType,
			},
			overwriteConfig: overwriteConfig{
				NewValues: map[string]string{
					"value_to_replace": "new-value",
				},
			},
			errorContains: "image variable: value_to_replace must be type string",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "tftest")
			assert.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			for file, content := range tc.tfFiles {
				err = os.WriteFile(path.Join(tmpDir, file), []byte(content), 0600)
				assert.NoError(t, err)
			}

			err = OverwriteTf(&tc.overwriteConfig, tmpDir)

			if tc.errorContains == "" {
				assert.NoError(t, err)

				actualContents, err := getDirContents(tmpDir)
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTfFiles, actualContents)
			} else {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.errorContains)
			}
		})
	}
}

func TestGetOverwriteConfig(t *testing.T) {
	testcases := []struct {
		name           string
		configBytes    []byte
		expectedConfig overwriteConfig
		errorContains  string
	}{{
		name:          "Invalid overwrite config shows parsing error",
		configBytes:   []byte("not valid json"),
		errorContains: "failure parsing overwrite config",
	}, {
		name: "Parses overwrite config",
		configBytes: []byte(`
{
	"variables": ["source_image"],
	"replacements": {"old_image": "new_image" },
	"newValues": {
		"source_image": "new_image"
	}
}
`),
		expectedConfig: overwriteConfig{
			Variables: []string{"source_image"},
			Replacements: map[string]string{
				"old_image": "new_image",
			},
			NewValues: map[string]string{
				"source_image": "new_image",
			},
		},
	},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := GetOverwriteConfig([]byte(tc.configBytes))
			if tc.errorContains == "" {
				assert.NoError(t, err)
				assert.Equal(t, &tc.expectedConfig, config)
			} else {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.errorContains)
			}
		})
	}
}

func TestOverwriteMetadata(t *testing.T) {
	testcases := []struct {
		name             string
		originalMetadata string
		expectedMetadata string
		overwriteConfig  overwriteConfig
		errorContains    string
	}{{
		name:             "Overwrite multiple variables",
		originalMetadata: metadata,
		expectedMetadata: metadataReplaced,
		overwriteConfig: overwriteConfig{
			Variables: []string{"source_image", "another_image"},
			Replacements: map[string]string{
				"old-image":   "new-image",
				"older-image": "newer-image",
			},
		},
	}, {
		name:             "Fail when metadata is invalid yaml",
		originalMetadata: "- not validyaml\ninvalid-",
		errorContains:    "failure parsing metadata.yaml",
	}, {
		name:             "Fail when variable not present in Metadata",
		originalMetadata: metadata,
		overwriteConfig: overwriteConfig{
			Variables: []string{"missing_variable"},
			Replacements: map[string]string{
				"original-value": "new-value",
			},
		},
		errorContains: "Missing valid default value for variable: missing_variable",
	}, {
		name:             "Fail when variable has no default value set",
		originalMetadata: metadataNoDefault,
		overwriteConfig: overwriteConfig{
			Variables: []string{"source_image"},
			Replacements: map[string]string{
				"original-value": "new-value",
			},
		},
		errorContains: "Missing valid default value for variable: source_image",
	}, {
		name:             "Fail when variable default value is not in replacements",
		originalMetadata: metadata,
		overwriteConfig: overwriteConfig{
			Variables: []string{"source_image"},
			Replacements: map[string]string{
				"non-existent": "new-value",
			},
		},
		errorContains: "default value: old-image of variable: source_image in metadata.yaml not found in replacements",
	}, {
		name:             "With NewValues, overwrite multiple variables",
		originalMetadata: metadata,
		expectedMetadata: metadataReplaced,
		overwriteConfig: overwriteConfig{
			NewValues: map[string]string{
				"source_image":  "new-image",
				"another_image": "newer-image",
			},
		},
	}, {
		name:             "With NewValues, ignores Variables and Replacements",
		originalMetadata: metadata,
		expectedMetadata: metadataReplaced,
		overwriteConfig: overwriteConfig{
			NewValues: map[string]string{
				"source_image":  "new-image",
				"another_image": "newer-image",
			},
			Variables: []string{"source_image", "another_image"},
			Replacements: map[string]string{
				"old-image":   "new-image-unused",
				"older-image": "newer-image-unused",
			},
		},
	}, {
		name:             "With NewValues, fail when variable is not present in Metadata",
		originalMetadata: metadata,
		overwriteConfig: overwriteConfig{
			NewValues: map[string]string{
				"missing_variable": "new-value",
			},
		},
		errorContains: "missing variable entry for variable: missing_variable",
	}, {
		name:             "With NewValues, adds default value when variable has no default value set",
		originalMetadata: metadataNoDefault,
		expectedMetadata: metadataDefaultAdded,
		overwriteConfig: overwriteConfig{
			NewValues: map[string]string{
				"source_image": "new-value",
			},
		},
	},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "tftest")
			assert.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			err = os.WriteFile(path.Join(tmpDir, "metadata.yaml"),
				[]byte(tc.originalMetadata), 0600)
			assert.NoError(t, err)

			err = OverwriteMetadata(&tc.overwriteConfig, tmpDir)

			if tc.errorContains == "" {
				assert.NoError(t, err)

				metadataBytes, err := os.ReadFile(path.Join(tmpDir, "metadata.yaml"))
				assert.NoError(t, err)
				actualMetadata := make(map[interface{}]interface{})
				expectedMetadata := make(map[interface{}]interface{})

				err = yaml.Unmarshal(metadataBytes, &actualMetadata)
				assert.NoError(t, err)

				err = yaml.Unmarshal([]byte(tc.expectedMetadata), expectedMetadata)
				assert.NoError(t, err)

				assert.Equal(t, expectedMetadata, actualMetadata)
			} else {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.errorContains)
			}
		})
	}
}

func TestOverwriteMetadataNoFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tftest")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = OverwriteMetadata(&overwriteConfig{}, tmpDir)
	assert.NoError(t, err)
}

func TestOverwiteMetadataPermissionError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tftest")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = os.WriteFile(path.Join(tmpDir, "metadata.yaml"), []byte("file"), 0111)
	assert.NoError(t, err)

	err = OverwriteMetadata(&overwriteConfig{}, tmpDir)
	assert.Error(t, err)
	assert.True(t, os.IsPermission(err))
}

func getDirContents(dir string) (map[string]string, error) {
	fileContents := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fileContents[path[len(dir)+1:]] = string(b)

		return nil
	})
	return fileContents, err
}

func TestOverwriteDisplay(t *testing.T) {
	testcases := []struct {
		name                    string
		originalMetadataDisplay string
		expectedMetadataDisplay string
		overwriteConfig         overwriteConfig
		errorContains           string
	}{
		{
			name:                    "Overwrite single display variable enum values",
			originalMetadataDisplay: metadataDisplayWithEnumsSingle,
			expectedMetadataDisplay: metadataDisplayWithEnumsSingleReplaced,
			overwriteConfig: overwriteConfig{
				Variables: []string{"source_image"},
				Replacements: map[string]string{
					"projects/click-to-deploy-images/global/images/wordpress-1": "projects/replacement/global/images/wordpress-1-new",
				},
			},
		},
		{
			name:                    "Overwrite multiple display variable enum values",
			originalMetadataDisplay: metadataDisplayWithEnumsDouble,
			expectedMetadataDisplay: metadataDisplayWithEnumsDoubleReplaced,
			overwriteConfig: overwriteConfig{
				Variables: []string{"source_image", "another_image"},
				Replacements: map[string]string{
					"projects/click-to-deploy-images/global/images/wordpress-1": "projects/replacement/global/images/wordpress-1-new",
					"projects/click-to-deploy-images/global/images/wordpress-2": "projects/replacement/global/images/wordpress-2-new",
					"projects/click-to-deploy-images/global/images/wordpress-3": "projects/replacement/global/images/wordpress-3-new",
				},
			},
		},
		{
			name:                    "No changes if no display variable enum value labels",
			originalMetadataDisplay: metadataDisplayNoEnums,
			expectedMetadataDisplay: metadataDisplayNoEnums,
			overwriteConfig: overwriteConfig{
				Variables:    []string{"source_image"},
				Replacements: map[string]string{},
			},
		}, {
			name:                    "Fail when metadata display is invalid yaml",
			originalMetadataDisplay: "- not validyaml\ninvalid-",
			errorContains:           "failure parsing metadata.display.yaml",
		}, {
			name:                    "Fail when display variable not present in Metadata display",
			originalMetadataDisplay: metadata,
			overwriteConfig: overwriteConfig{
				Variables: []string{"missing_variable"},
				Replacements: map[string]string{
					"original-value": "new-value",
				},
			},
			errorContains: "missing valid display info for variable: missing_variable",
		}, {
			name:                    "Fail when display variable enum value is not in replacements",
			originalMetadataDisplay: metadataDisplayWithEnumsSingle,
			overwriteConfig: overwriteConfig{
				Variables: []string{"source_image"},
				Replacements: map[string]string{
					"non-existent": "new-value",
				},
			},
			errorContains: "enum value: projects/click-to-deploy-images/global/images/wordpress-1 of variable: source_image in metadata.display.yaml not found in replacements",
		}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "tftest")
			assert.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			err = os.WriteFile(path.Join(tmpDir, "metadata.display.yaml"),
				[]byte(tc.originalMetadataDisplay), 0600)
			assert.NoError(t, err)

			err = OverwriteDisplay(&tc.overwriteConfig, tmpDir)

			if tc.errorContains == "" {
				assert.NoError(t, err)

				metadataDisplayBytes, err := os.ReadFile(path.Join(tmpDir, "metadata.display.yaml"))
				assert.NoError(t, err)
				actualMetadataDisplay := make(map[interface{}]interface{})
				expectedMetadataDisplay := make(map[interface{}]interface{})

				err = yaml.Unmarshal(metadataDisplayBytes, &actualMetadataDisplay)
				assert.NoError(t, err)

				err = yaml.Unmarshal([]byte(tc.expectedMetadataDisplay), expectedMetadataDisplay)
				assert.NoError(t, err)

				assert.Equal(t, expectedMetadataDisplay, actualMetadataDisplay)
			} else {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.errorContains)
			}
		})
	}
}

var mainTf string = `
resource "google_compute_instance_template" "template" {
  name = "template"
}

variable "value_to_replace" {
  type    = string
  default = "original-value"
}

variable "other_value_to_replace" {
  type    = string
  default = "old-value"
}
`

var mainTfReplaced string = `
resource "google_compute_instance_template" "template" {
  name = "template"
}

variable "value_to_replace" {
  type    = string
  default = "new-value"
}

variable "other_value_to_replace" {
  type    = string
  default = "newer-value"
}
`

var mainTfNoLabel string = `
provider "google" {
  project = var.project_id
}

resource "google_compute_instance_template" "template" {
  name = "template"
}

variable "value_to_replace" {
  type    = string
  default = "original-value"
}
`

var mainTfProvidedLabel string = `
provider "google" {
  project = var.project_id
  default_labels {
    goog-partner-solution = "new-consumer-label"
  }
}

resource "google_compute_instance_template" "template" {
  name = "template"
}

variable "value_to_replace" {
  type    = string
  default = "original-value"
}
`

var mainTfLabelUpserted string = `
provider "google" {
  project = var.project_id
  default_labels {
    goog-partner-solution = "new-consumer-label"
  }
}

resource "google_compute_instance_template" "template" {
  name = "template"
}

variable "value_to_replace" {
  type    = string
  default = "new-value"
}
`

var otherTf string = `
variable "another_variable" {
  type = string
  default = "oldest-value"
}
`

var otherTfReplaced string = `
variable "another_variable" {
  type    = string
  default = "newest-value"
}
`

var tfNoDefault string = `
variable "value_to_replace" {
  type = string
}
`

var tfDefaultAdded string = `
variable "value_to_replace" {
  type    = string
  default = "new-value"
}
`

var tfDefaultWrongType string = `
variable "value_to_replace" {
  type = map(number)
  default = {
    foo = 2
    bar = 4
  }
}
`
var tfNoDefaultWrongType string = `
variable "value_to_replace" {
  type = map(number)
}
`

var metadata string = `
spec:
  interfaces:
    variables:
    - name: source_image
      description: The image name for the disk for the VM instance.
      varType: string
      defaultValue: old-image
    - name: another_image
      description: The image name for the disk for the VM instance.
      varType: string
      defaultValue: older-image
`

var metadataReplaced string = `
spec:
  interfaces:
    variables:
    - name: source_image
      description: The image name for the disk for the VM instance.
      varType: string
      defaultValue: new-image
    - name: another_image
      description: The image name for the disk for the VM instance.
      varType: string
      defaultValue: newer-image
`

var metadataNoDefault string = `
spec:
  interfaces:
    variables:
    - name: source_image
      description: The image name for the disk for the VM instance.
      varType: string
`

var metadataDefaultAdded string = `
spec:
  interfaces:
    variables:
    - name: source_image
      description: The image name for the disk for the VM instance.
      varType: string
      defaultValue: new-value
`

var metadataDisplayWithEnumsSingle string = `
spec:
  ui:
    input:
      variables:
        source_image:
          name: source_image
          title: Source Image
          enumValueLabels:
            - label: wordpress-1
              value: projects/click-to-deploy-images/global/images/wordpress-1
          xGoogleProperty:
            type: ET_GCE_DISK_IMAGE
`

var metadataDisplayWithEnumsSingleReplaced string = `
spec:
  ui:
    input:
      variables:
        source_image:
          name: source_image
          title: Source Image
          enumValueLabels:
            - label: wordpress-1
              value: projects/replacement/global/images/wordpress-1-new
          xGoogleProperty:
            type: ET_GCE_DISK_IMAGE
`

var metadataDisplayWithEnumsDouble string = `
spec:
  ui:
    input:
      variables:
        source_image:
          name: source_image
          title: Source Image
          enumValueLabels:
            - label: wordpress-1
              value: projects/click-to-deploy-images/global/images/wordpress-1
            - label: wordpress-2
              value: projects/click-to-deploy-images/global/images/wordpress-2
          xGoogleProperty:
            type: ET_GCE_DISK_IMAGE
        another_image:
          name: another_image
          title: Another Image
          enumValueLabels:
            - label: wordpress-3
              value: projects/click-to-deploy-images/global/images/wordpress-3
          xGoogleProperty:
            type: ET_GCE_DISK_IMAGE
`

var metadataDisplayWithEnumsDoubleReplaced string = `
spec:
  ui:
    input:
      variables:
        source_image:
          name: source_image
          title: Source Image
          enumValueLabels:
            - label: wordpress-1
              value: projects/replacement/global/images/wordpress-1-new
            - label: wordpress-2
              value: projects/replacement/global/images/wordpress-2-new
          xGoogleProperty:
            type: ET_GCE_DISK_IMAGE
        another_image:
          name: another_image
          title: Another Image
          enumValueLabels:
            - label: wordpress-3
              value: projects/replacement/global/images/wordpress-3-new
          xGoogleProperty:
            type: ET_GCE_DISK_IMAGE
`

var metadataDisplayNoEnums string = `
spec:
  ui:
    input:
      variables:
        source_image:
          name: source_image
          title: Source Image
          xGoogleProperty:
            type: ET_GCE_DISK_IMAGE
`
