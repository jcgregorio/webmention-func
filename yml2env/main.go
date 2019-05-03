// Exports the values in the env.yaml file in a way that allows
// setting them in environment variables in a bash script.
//
// E.g.:
//
//    $(go run ./yml2env)
package main

import (
	"fmt"
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"
)

func main() {
	b, err := ioutil.ReadFile("env.yaml")
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	cfg := map[string]string{}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		log.Fatalf("Not a valid yaml file: %s", err)
	}
	for key, value := range cfg {
		fmt.Printf("export %s=%s\n", key, value)
	}
}
