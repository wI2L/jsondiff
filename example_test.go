package jsondiff_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/wI2L/jsondiff"
)

type (
	Pod struct {
		Spec PodSpec `json:"spec,omitempty"`
	}
	PodSpec struct {
		Containers []Container `json:"containers,omitempty"`
		Volumes    []Volume    `json:"volumes,omitempty"`
	}
	Container struct {
		Name         string        `json:"name"`
		Image        string        `json:"image,omitempty"`
		VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`
	}
	Volume struct {
		Name         string `json:"name"`
		VolumeSource `json:",inline"`
	}
	VolumeSource struct {
		EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	}
	VolumeMount struct {
		Name      string `json:"name"`
		MountPath string `json:"mountPath"`
	}
	EmptyDirVolumeSource struct {
		Medium StorageMedium `json:"medium,omitempty"`
	}
	StorageMedium string
)

const (
	StorageMediumDefault StorageMedium = ""
	StorageMediumMemory  StorageMedium = "Memory"
)

func ExampleCompare() {
	createPod := func() Pod {
		return Pod{
			Spec: PodSpec{
				Containers: []Container{{
					Name:  "webserver",
					Image: "nginx:latest",
					VolumeMounts: []VolumeMount{{
						Name:      "shared-data",
						MountPath: "/usr/share/nginx/html",
					}},
				}},
				Volumes: []Volume{{
					Name: "shared-data",
					VolumeSource: VolumeSource{
						EmptyDir: &EmptyDirVolumeSource{
							Medium: StorageMediumMemory,
						},
					},
				}},
			},
		}
	}
	oldPod := createPod()
	newPod := createPod()

	newPod.Spec.Containers[0].Image = "nginx:1.19.5-alpine"
	newPod.Spec.Volumes[0].EmptyDir.Medium = StorageMediumDefault

	patch, err := jsondiff.Compare(oldPod, newPod)
	if err != nil {
		log.Fatal(err)
	}
	for _, op := range patch {
		fmt.Printf("%s\n", op)
	}
	// Output:
	// {"op":"replace","path":"/spec/containers/0/image","value":"nginx:1.19.5-alpine"}
	// {"op":"remove","path":"/spec/volumes/0/emptyDir/medium"}
}

func ExampleCompareJSON() {
	type Phone struct {
		Type   string `json:"type"`
		Number string `json:"number"`
	}
	type Person struct {
		Firstname string  `json:"firstName"`
		Lastname  string  `json:"lastName"`
		Gender    string  `json:"gender"`
		Age       int     `json:"age"`
		Phones    []Phone `json:"phoneNumbers"`
	}
	source, err := ioutil.ReadFile("testdata/examples/john.json")
	if err != nil {
		log.Fatal(err)
	}
	var john Person
	if err := json.Unmarshal(source, &john); err != nil {
		log.Fatal(err)
	}
	john.Age = 30
	john.Phones = append(john.Phones, Phone{
		Type:   "mobile",
		Number: "209-212-0015",
	})
	target, err := json.Marshal(john)
	if err != nil {
		log.Fatal(err)
	}
	patch, err := jsondiff.CompareJSON(source, target)
	if err != nil {
		log.Fatal(err)
	}
	for _, op := range patch {
		fmt.Printf("%s\n", op)
	}
	// Output:
	// {"op":"replace","path":"/age","value":30}
	// {"op":"add","path":"/phoneNumbers/-","value":{"number":"209-212-0015","type":"mobile"}}
}

func ExampleInvertible() {
	source := `{"a":"1","b":"2"}`
	target := `{"a":"3","c":"4"}`

	patch, err := jsondiff.CompareJSONOpts(
		[]byte(source),
		[]byte(target),
		jsondiff.Invertible(),
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, op := range patch {
		fmt.Printf("%s\n", op)
	}
	// Output:
	// {"op":"test","path":"/a","value":"1"}
	// {"op":"replace","path":"/a","value":"3"}
	// {"op":"test","path":"/b","value":"2"}
	// {"op":"remove","path":"/b"}
	// {"op":"add","path":"/c","value":"4"}
}

func ExampleFactorize() {
	source := `{"a":[1,2,3],"b":{"foo":"bar"}}`
	target := `{"a":[1,2,3],"c":[1,2,3],"d":{"foo":"bar"}}`

	patch, err := jsondiff.CompareJSONOpts(
		[]byte(source),
		[]byte(target),
		jsondiff.Factorize(),
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, op := range patch {
		fmt.Printf("%s\n", op)
	}
	// Output:
	// {"op":"copy","from":"/a","path":"/c"}
	// {"op":"move","from":"/b","path":"/d"}
}
