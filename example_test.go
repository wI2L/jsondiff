package jsondiff_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/wI2L/jsondiff"
	corev1 "k8s.io/api/core/v1"
)

func ExampleCompare() {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "webserver",
				Image: "nginx:latest",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "shared-data",
					MountPath: "/usr/share/nginx/html",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "shared-data",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}},
		},
	}
	newPod := pod.DeepCopy()

	newPod.Spec.Containers[0].Image = "nginx:1.19.5-alpine"
	newPod.Spec.Volumes[0].EmptyDir.Medium = corev1.StorageMediumDefault

	patch, err := jsondiff.Compare(pod, newPod)
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
