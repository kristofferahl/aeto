package template

import (
	"fmt"
	"strconv"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
)

type Data struct {
	Key                string
	ResourceNamePrefix string
	Name               string
	Namespaces         Namespaces
	Labels             map[string]string
	Annotations        map[string]string
	Parameters         []*corev1alpha1.Parameter
}

type Namespaces struct {
	Tenant   string
	Operator string
}

// String returns the value of a named string parameter
func (d Data) String(name string) (string, error) {
	for _, p := range d.Parameters {
		if p.Name == name {
			return p.Value()
		}
	}
	return "", fmt.Errorf("parameter %s not found", name)
}

// Int returns the value of a named int parameter
func (d Data) Int(name string) (int, error) {
	v, err := d.String(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(v)
}

// Bool returns the value of a named bool parameter
func (d Data) Bool(name string) (bool, error) {
	v, err := d.String(name)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(v)
}
