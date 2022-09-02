package template

import (
	"fmt"
	"strconv"
	"strings"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
)

type Data struct {
	Name         string
	PrefixedName string
	DisplayName  string
	Namespaces   Namespaces
	Labels       map[string]string
	Annotations  map[string]string
	Parameters   []*corev1alpha1.Parameter
	Utils        UtilityFunctions
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

type UtilityFunctions struct {
	String
	Slice
}

type String struct{}

func (d String) Replace(s, old, new string, n int) string {
	return strings.Replace(s, old, new, n)
}

func (d String) ReplaceAll(s string, old string, new string) string {
	return strings.ReplaceAll(s, old, new)
}

type Slice struct{}

func (d Slice) ContainsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
