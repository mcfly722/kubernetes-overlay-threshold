// docker overlay path:

//
// /var/lib/docker/image/overlay2/layerdb/mounts/<containerId>/mount-id
// contains mapping containerId -> mountContainerId
// /var/lib/docker/overlay2/<mountContainerId>/
// contains all container files

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type k8s struct {
	clientset kubernetes.Interface
}

func newK8s() (*k8s, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client := k8s{}

	client.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func getMountIdforContainerID(dockerPath string, containerId string) (string, error) {
	bytes, err := ioutil.ReadFile(fmt.Sprintf("%v/image/overlay2/layerdb/mounts/%v/mount-id", dockerPath, containerId))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func dirSize(path string) (uint64, error) {
	var size uint64
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += uint64(info.Size())
		}
		return err
	})
	return size, err
}

func main() {
	fmt.Println("v12")
	var sleepIntervalSecFlag *int
	var dockerPathFlag *string

	sleepIntervalSecFlag = flag.Int("sleepIntervalSec", 20, "interval between containers directory parsing")
	dockerPathFlag = flag.String("dockerPath", "/var/lib/docker", "containers directory")

	flag.Parse()

	fmt.Println(fmt.Sprintf("sleepIntervalSec    = %v", *sleepIntervalSecFlag))
	fmt.Println(fmt.Sprintf("dockerPath = %v", *dockerPathFlag))

	containerMounts := map[string]string{}

	k8s, err := newK8s()
	if err != nil {
		panic(err)
	}

	for {

		pods, err := k8s.clientset.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			fmt.Printf(fmt.Sprintf("%v", err.Error()))
		} else {
			for _, pod := range pods.Items {

				if len(pod.Status.ContainerStatuses) > 0 {
					for container := range pod.Spec.Containers {
						fullContainerID := pod.Status.ContainerStatuses[container].ContainerID
						if len(fullContainerID) > 10 {
							containerID := strings.Replace(fullContainerID, "docker://", "", -1)

							_, ok := containerMounts[containerID]
							if !ok {
								mountID, err := getMountIdforContainerID(*dockerPathFlag, containerID)
								if err == nil {
									containerMounts[containerID] = mountID
								}
							}

							mountID, ok := containerMounts[containerID]
							if ok {
								fullContainerName := fmt.Sprintf("%v:%v\\%v", pod.Namespace, pod.Name, pod.Spec.Containers[container].Name)
								size, err := dirSize(fmt.Sprintf("%v/overlay2/%v/diff", *dockerPathFlag, mountID))
								if err != nil {
									fmt.Println(fmt.Sprintf("error: could not parse directory for %v`n%v", fullContainerName, err))
								} else {
									fmt.Println(fmt.Sprintf("%8vMB  %v", size/(1024*1024), fullContainerName))
								}
							}
						}
					}
				}
			}
		}

		time.Sleep(time.Duration(*sleepIntervalSecFlag) * time.Second)
		fmt.Println("")
	}

}
