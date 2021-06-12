package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"log"
	"time"
)

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
	sleepIntervalSecFlag = flag.Int("sleepIntervalSec", 4, "interval between containers directory parsing")

	flag.Parse()

	fmt.Println(fmt.Sprintf("sleepIntervalSec = %v", *sleepIntervalSecFlag))



	for {
		directories := map[string]uint64{}

		files, err := ioutil.ReadDir("./")
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
			fmt.Println(fmt.Sprintf("%10v %v",size,name))
		}

		time.Sleep(time.Duration(*sleepIntervalSecFlag) * time.Second)
		fmt.Println("")
	}

}
