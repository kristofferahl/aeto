package tenant

import (
	"encoding/json"
	"fmt"

	"github.com/PaesslerAG/jsonpath"
	"github.com/kristofferahl/aeto/internal/pkg/common"
	"github.com/kristofferahl/aeto/internal/pkg/convert"
	"k8s.io/apimachinery/pkg/runtime"
)

type ResourceGroup struct {
	Name           string       `json:"name"`
	SourceTemplate string       `json:"sourceTemplate"`
	Resources      ResourceList `json:"resources"`
}

type ResourceGroupList []ResourceGroup

type Resource struct {
	Id       string           `json:"id"`
	Order    int              `json:"order"`
	Sum      string           `json:"sum"`
	Embedded EmbeddedResource `json:"embedded"`
}

type ResourceList []Resource

type EmbeddedResource struct {
	runtime.RawExtension `json:",inline"`
}

func (rgl ResourceGroupList) Resources() ResourceList {
	rl := ResourceList{}
	for _, rg := range rgl {
		rl = append(rl, rg.Resources...)
	}
	return rl
}

func (g *ResourceGroup) JsonPath(path string) (string, error) {
	bytes, err := json.Marshal(g)
	if err != nil {
		return "", err
	}

	v := interface{}(nil)
	json.Unmarshal(bytes, &v)

	value, err := jsonpath.Get(fmt.Sprintf("$%s", path), v)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s", value), nil
}

func (rl ResourceList) Find(id string) (index int, existing *Resource) {
	index = -1
	for i, r := range rl {
		idEqual := r.Id == id
		if idEqual {
			existing = &r
			index = i
			break
		}
	}
	return
}

func (rl ResourceList) Remove(index int) ResourceList {
	if index < 0 {
		return rl
	}
	return append(rl[:index], rl[index+1:]...)
}

func (r Resource) ResourceIdentifier() (common.ResourceIdentifier, error) {
	return convert.RawExtensionToResourceIdentifier(r.Embedded.RawExtension)
}
