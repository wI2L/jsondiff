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
	// {"value":"nginx:1.19.5-alpine","op":"replace","path":"/spec/containers/0/image"}
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
	source, err := os.ReadFile("testdata/examples/person.json")
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
	// {"value":30,"op":"replace","path":"/age"}
	// {"value":{"number":"209-212-0015","type":"mobile"},"op":"add","path":"/phoneNumbers/-"}
}

func ExampleInvertible() {
	source := `{"a":"1","b":"2"}`
	target := `{"a":"3","c":"4"}`

	patch, err := jsondiff.CompareJSON(
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
	// {"value":"1","op":"test","path":"/a"}
	// {"value":"3","op":"replace","path":"/a"}
	// {"value":"2","op":"test","path":"/b"}
	// {"op":"remove","path":"/b"}
	// {"value":"4","op":"add","path":"/c"}
}

func ExampleFactorize() {
	source := `{"a":[1,2,3],"b":{"foo":"bar"}}`
	target := `{"a":[1,2,3],"c":[1,2,3],"d":{"foo":"bar"}}`

	patch, err := jsondiff.CompareJSON(
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

	patch, err := jsondiff.CompareJSON(
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

	patch, err := jsondiff.Compare(
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
	// {"value":"nginx","op":"replace","path":"/spec/containers/0/name"}
	// {"value":"data","op":"replace","path":"/spec/volumes/0/name"}
}

func ExampleUnmarshalFunc() {
	source := `{"A":"bar","B":3.14,"C":false}`
	target := `{"A":"baz","B":3.14159,"C":true}`

	patch, err := jsondiff.CompareJSON(
		[]byte(source),
		[]byte(target),
		jsondiff.UnmarshalFunc(func(b []byte, v any) error {
			dec := json.NewDecoder(bytes.NewReader(b))
			dec.UseNumber()
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
	// {"value":"baz","op":"replace","path":"/A"}
	// {"value":3.14159,"op":"replace","path":"/B"}
	// {"value":true,"op":"replace","path":"/C"}
}

func ExampleMergePatch() {
	src := map[string]interface{}{
		"foo": "baz",
		"bar": []string{"a", "b", "c"},
		"baz": 3.14159,
	}
	tgt := map[string]interface{}{
		"foo": "bar",
		"bar": []string{"y", "y", "z"},
	}
	patch, err := jsondiff.MergePatch(src, tgt)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(patch))
	// Output:
	// {"bar":["y","y","z"],"baz":null,"foo":"bar"}
}

func ExampleMergePatchJSON() {
	src := `{"a":[1,2,3],"b":{"foo":"bar"}}`
	tgt := `{"a":[1,2,3],"c":[1,2,3],"d":{"foo":"bar"}}`

	patch, err := jsondiff.MergePatchJSON([]byte(src), []byte(tgt))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(patch))
	// Output:
	// {"b":null,"c":[1,2,3],"d":{"foo":"bar"}}
}
