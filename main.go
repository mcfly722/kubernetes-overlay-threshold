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

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
)

type k8s struct {
	clientset kubernetes.Interface
}

func eventRecorder(
	kubeClient *kubernetes.Clientset) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: "controlplane"})
	return recorder
}

func newK8s() (*k8s, record.EventRecorder, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, err
	}

	client := k8s{}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	client.clientset = kubeClient

	eventRecorder := eventRecorder(kubeClient)

	return &client, eventRecorder, nil
}

func getMountIdforContainerID(dockerPath string, containerId string) (string, error) {
	bytes, err := ioutil.ReadFile(fmt.Sprintf("%v/image/overlay2/layerdb/mounts/%v/mount-id", dockerPath, containerId))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func dirSize(path string) (uint64, uint64, error) {
	var size uint64
	var count uint64
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += uint64(info.Size())
		}
		count++
		return err
	})
	return size, count, err
}

func main() {
	var sleepIntervalSecFlag *int
	var dockerPathFlag *string
	var overlayThresholdMBFlag *int
	var overlayFilesThresholdFlag *uint64

	sleepIntervalSecFlag = flag.Int("sleepIntervalSec", 20, "interval between containers directory parsing")
	dockerPathFlag = flag.String("dockerPath", "/var/lib/docker", "containers directory")
	overlayThresholdMBFlag = flag.Int("overlayThresholdMB", 4096, "overlay threshold in MB")
	overlayFilesThresholdFlag = flag.Uint64("maxFilesThreshold", 1024*1024, "overlay maximum files threshold")

	flag.Parse()

	fmt.Println(fmt.Sprintf("sleepIntervalSec   = %v", *sleepIntervalSecFlag))
	fmt.Println(fmt.Sprintf("dockerPath         = %v", *dockerPathFlag))
	fmt.Println(fmt.Sprintf("overlayThresholdMB = %v", *overlayThresholdMBFlag))
	fmt.Println(fmt.Sprintf("maxFilesThreshold  = %v", *overlayFilesThresholdFlag))

	containerMounts := map[string]string{}

	k8s, eventRecorder, err := newK8s()
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
								size, filesCount, err := dirSize(fmt.Sprintf("%v/overlay2/%v/diff", *dockerPathFlag, mountID))
								if err != nil {
									fmt.Println(fmt.Sprintf("error: could not parse directory for %v`n%v", fullContainerName, err))
								} else {
									fmt.Println(fmt.Sprintf("%8vMB  %8v  %v", size/(1024*1024), filesCount, fullContainerName))
								}

								toDelete := false

								if size/(1024*1024) > uint64(*overlayThresholdMBFlag) {

									ref, err := reference.GetReference(scheme.Scheme, &pod)
									if err != nil {
										fmt.Println(fmt.Sprintf("error: %v (%v)", err, fullContainerName))
									}
									msg := fmt.Sprintf("Container %v exceeded %vMB threshold", pod.Spec.Containers[container].Name, uint64(*overlayThresholdMBFlag))
									fmt.Println(msg)
									eventRecorder.Event(ref, v1.EventTypeWarning, "Killing", msg)
									toDelete = true
								}

								if filesCount > uint64(*overlayFilesThresholdFlag) {

									ref, err := reference.GetReference(scheme.Scheme, &pod)
									if err != nil {
										fmt.Println(fmt.Sprintf("error: %v (%v)", err, fullContainerName))
									}
									msg := fmt.Sprintf("Container %v exceeded %v files threshold", pod.Spec.Containers[container].Name, uint64(*overlayFilesThresholdFlag))
									fmt.Println(msg)
									eventRecorder.Event(ref, v1.EventTypeWarning, "Killing", msg)
									toDelete = true
								}

								// try to drop Pod
								if toDelete == true {
									if err := k8s.clientset.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
										fmt.Println(fmt.Sprintf("Pod %v:%v deleting error: %v", pod.Namespace, pod.Name, err))
									} else {
										fmt.Println(fmt.Sprintf("Pod %v:%v deleted successfully", pod.Namespace, pod.Name))
									}
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
