package jsondiff_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"

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

func createPod() Pod {
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

func ExampleCompare() {
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

func ExampleCompareOpts() {
	oldPod := createPod()
	newPod := createPod()

	newPod.Spec.Volumes = append(newPod.Spec.Volumes, oldPod.Spec.Volumes[0])

	patch, err := jsondiff.CompareOpts(
		oldPod,
		newPod,
		jsondiff.Factorize(),
		jsondiff.Rationalize(),
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, op := range patch {
		fmt.Printf("%s\n", op)
	}
	// Output:
	// {"op":"copy","from":"/spec/volumes/0","path":"/spec/volumes/-"}
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
	source, err := os.ReadFile("testdata/examples/john.json")
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

func ExampleIgnores() {
	source := `{"A":"bar","B":"baz","C":"foo"}`
	target := `{"A":"rab","B":"baz","D":"foo"}`

	patch, err := jsondiff.CompareJSONOpts(
		[]byte(source),
		[]byte(target),
		jsondiff.Ignores("/A", "/C", "/D"),
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, op := range patch {
		fmt.Printf("%s\n", op)
	}
	// Output:
}

func ExampleMarshalFunc() {
	oldPod := createPod()
	newPod := createPod()

	newPod.Spec.Containers[0].Name = "nginx"
	newPod.Spec.Volumes[0].Name = "data"

	patch, err := jsondiff.CompareOpts(
		oldPod,
		newPod,
		jsondiff.MarshalFunc(func(v any) ([]byte, error) {
			buf := bytes.Buffer{}
			enc := json.NewEncoder(&buf)
			err := enc.Encode(v)
			if err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, op := range patch {
		fmt.Printf("%s\n", op)
	}
	// Output:
	// {"op":"replace","path":"/spec/containers/0/name","value":"nginx"}
	// {"op":"replace","path":"/spec/volumes/0/name","value":"data"}
}

func ExampleUnmarshalFunc() {
	source := `{"A":"bar","B":3.14,"C":false}`
	target := `{"A":"baz","B":3.14159,"C":true}`

	patch, err := jsondiff.CompareJSONOpts(
		[]byte(source),
		[]byte(target),
		jsondiff.UnmarshalFunc(func(b []byte, v any) error {
			dec := json.NewDecoder(bytes.NewReader(b))
			return dec.Decode(v)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, op := range patch {
		fmt.Printf("%s\n", op)
	}
	// Output:
	// {"op":"replace","path":"/A","value":"baz"}
	// {"op":"replace","path":"/B","value":3.14159}
	// {"op":"replace","path":"/C","value":true}
}
