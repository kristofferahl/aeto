package convert

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	yamlghodss "github.com/ghodss/yaml"
)

var yamlSeparator = regexp.MustCompile(`\n---`)

// YamlToUnstructuredSlice splits a YAML document into unstructured objects
func YamlToUnstructuredSlice(source string) ([]*unstructured.Unstructured, error) {
	parts := yamlSeparator.Split(source, -1)
	var objs []*unstructured.Unstructured
	var firstErr error
	for _, part := range parts {
		var objMap map[string]interface{}
		err := yamlghodss.Unmarshal([]byte(part), &objMap)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("Failed to unmarshal manifest: %v", err)
			}
			continue
		}
		if len(objMap) == 0 {
			// handles case where theres no content between `---`
			continue
		}
		var obj unstructured.Unstructured
		err = yamlghodss.Unmarshal([]byte(part), &obj)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("Failed to unmarshal manifest: %v", err)
			}
			continue
		}
		objs = append(objs, &obj)
	}
	return objs, firstErr
}

// YamlToStringSlice splits a YAML document into byte slices objects
func YamlToStringSlice(source string) ([]string, error) {
	dec := yaml.NewDecoder(bytes.NewReader([]byte(source)))

	var res []string
	for {
		var value interface{}
		err := dec.Decode(&value)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		valueBytes, err := yaml.Marshal(value)
		if err != nil {
			return nil, err
		}
		res = append(res, string(valueBytes))
	}

	return res, nil
}
