package template

import (
	"bytes"
	"fmt"
	"text/template"

	yaml "github.com/ghodss/yaml"
)

const (
	InputFormatJson InputFormat = "json"
	InputFormatYaml InputFormat = "yaml"
)

type yamlTemplate struct {
	format InputFormat
	yaml   string
}

type InputFormat string

func NewYamlTemplate(template string, format InputFormat) (yamlTemplate, error) {
	yamlString := template

	if format == InputFormatJson {
		// TODO: Is there a better way then converting to yaml before applying template? JSON caused issues with function calls!
		b, err := yaml.JSONToYAML([]byte(template))
		if err != nil {
			return yamlTemplate{}, err
		}
		yamlString = string(b)
	}

	return yamlTemplate{
		format: format,
		yaml:   yamlString,
	}, nil
}

// Execute parses and executes a yaml template given the specified data
func (t yamlTemplate) Execute(data Data) (string, error) {
	tmpl, err := template.New("resource").Parse(t.yaml)
	if err != nil {
		return "", fmt.Errorf("failed to parse template, %v", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template, %v", err)
	}

	str := buf.String()
	return str, nil
}
