// docker overlay path:
// /var/lib/docker/image/overlay2/layerdb/mounts


package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"log"
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

func dirSize(path string) (uint64, error) {
	var size uint64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
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

	var sleepIntervalSecFlag *int
	var containersDirectoryFlag *string

	sleepIntervalSecFlag = flag.Int("sleepIntervalSec", 20, "interval between containers directory parsing")
	containersDirectoryFlag = flag.String("containersDirectory", "./", "containers directory")


	flag.Parse()

	fmt.Println(fmt.Sprintf("sleepIntervalSec    = %v", *sleepIntervalSecFlag))
	fmt.Println(fmt.Sprintf("containersDirectory = %v", *containersDirectoryFlag))

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
				fmt.Printf(fmt.Sprintf("pod: %v", pod.GetName()))
				for container := range pod.Spec.Containers {
					fmt.Printf(fmt.Sprintf("   %v", pod.Spec.Containers[container].Name))
				}
			}
		}




		directories := map[string]uint64{}

		files, err := ioutil.ReadDir(*containersDirectoryFlag)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {

			if file.IsDir() {

				size, err := dirSize(file.Name())

				if err != nil {
					fmt.Println(fmt.Sprintf("error parsing %s: %s", file.Name(), err))
				} else {
					directories[file.Name()] = size
				}
			}
		}

		for name, size := range directories {
			fmt.Println(fmt.Sprintf("%20vMB %v",size/(1024*1024),name))
		}

		time.Sleep(time.Duration(*sleepIntervalSecFlag) * time.Second)
		fmt.Println("")
	}

}
