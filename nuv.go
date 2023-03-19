// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	docopt "github.com/docopt/docopt-go"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

func help() {
	if exists(".", NUVOPTS) {
		fmt.Println(readfile(NUVOPTS))
	} else {
		//fmt.Println("-t", "Nuvfile", "-l")
		Task("-t", NUVFILE, "-l")
	}
}

// parseArgs parse the arguments acording the docopt
// it returns a sequence suitable to be feed as arguments for task.
// note that it will change hyphens for flags ('-c', '--count') to '_' ('_c' '__count')
// and '<' and '>' for parameters '_' (<hosts> => _hosts_)
// boolean are "true" or "false" and arrays in the form ('first' 'second')
// suitable to be used as arrays
// Examples:
// if "Usage: nettool ping [--count=<max>] <hosts>..."
// with "ping --count=3 google apple" returns
// ping=true _count=3 _hosts_=('google' 'apple')
func parseArgs(usage string, args []string) []string {
	res := []string{}
	// parse args
	parser := docopt.Parser{}
	opts, err := parser.ParseArgs(usage, args, NuvVersion)
	if err != nil {
		warn(err)
		return res
	}
	for k, v := range opts {
		kk := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(k, "-", "_"), "<", "_"), ">", "_")
		vv := ""
		//fmt.Println(v, reflect.TypeOf(v))
		switch o := v.(type) {
		case bool:
			vv = "false"
			if o {
				vv = "true"
			}
		case string:
			vv = o
		case []string:
			a := []string{}
			for _, i := range o {
				a = append(a, fmt.Sprintf("'%v'", i))
			}
			vv = "(" + strings.Join(a, " ") + ")"
		case nil:
			vv = ""
		}
		res = append(res, fmt.Sprintf("%s=%s", kk, vv))
	}
	sort.Strings(res)
	return res
}

// Nuv parse args moving
// into the folder corresponding to args
// then parse them with docopts and invokes task
func Nuv(base string, args []string) error {
	// go down using args as subcommands
	err := os.Chdir(base)
	if err != nil {
		return err
	}
	rest := args

	for _, task := range args {
		trace("task", task)
		// try to correct name if it's a prefix
		taskName, err := validateTaskName(task)
		if err != nil {
			return err
		}

		// if valid, check if it's a folder and move to it
		if isDir(taskName) && exists(taskName, NUVFILE) {
			os.Chdir(taskName)
			//remove it from the args
			// rest = append(rest[:i], rest[i+1:]...)
			rest = rest[1:]
		} else {
			// stop when non folder reached
			break
		}
	}

	if len(rest) == 0 || rest[0] == "help" {
		help()
		return nil
	}

	// parsed args
	if exists(".", NUVOPTS) {
		//fmt.Println("PREPARSE:", rest)
		parsedArgs := parseArgs(readfile(NUVOPTS), rest)
		prefix := []string{"-t", NUVFILE}
		if len(rest) > 0 && rest[0][0] != '-' {
			prefix = append(prefix, rest[0])
		}
		parsedArgs = append(prefix, parsedArgs...)
		//fmt.Println("POSTPARSE:", parsedArgs)
		Task(parsedArgs...)
		return nil
	}

	// get first string without '=' from rest, it's the task name
	idx := 0
	for i, s := range rest {
		if !strings.Contains(s, "=") {
			idx = i
			break
		}
	}

	mainTask := rest[idx]

	// unparsed args - separate variable assignments from extra args
	pre := []string{"-t", NUVFILE, mainTask}
	post := []string{"--"}
	for _, s := range rest[1:] {
		if strings.Contains(s, "=") {
			pre = append(pre, s)
		} else {
			post = append(post, s)
		}
	}
	taskArgs := append(pre, post...)
	debug(taskArgs)
	Task(taskArgs...)
	return nil
}

// validateTaskName does the following:
// 1. Check that the given task name is found in the nuvfile.yaml and return it
// 2. If not found, check if the input is a prefix of any task name, if it is for only one return the proper task name
// 3. If the prefix is valid for more than one task, return an error
// 4. If the prefix is not valid for any task, return an error
func validateTaskName(name string) (string, error) {
	pwd, _ := os.Getwd()

	candidates := []string{}
	tasks := getTaskNamesList(pwd)
	if !slices.Contains(tasks, "help") {
		tasks = append(tasks, "help")
	}
	for _, t := range tasks {
		if t == name {
			return name, nil
		}
		if strings.HasPrefix(t, name) {
			candidates = append(candidates, t)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no task named %s found", name)
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	return "", fmt.Errorf("ambiguous task: %s. Possible tasks: %v", name, candidates)
}

// obtains the task names from the nuvfile.yaml inside the given directory
func getTaskNamesList(dir string) []string {
	m := make(map[interface{}]interface{})
	if exists(dir, NUVFILE) {
		dat, err := os.ReadFile(joinpath(dir, NUVFILE))
		if err != nil {
			return make([]string, 0)
		}

		err = yaml.Unmarshal(dat, &m)
		if err != nil {
			warn("error reading nuvfile.yml")
			return make([]string, 0)
		}
		tasksMap, ok := m["tasks"].(map[string]interface{})
		if !ok {
			warn("error checking task list, perhaps no tasks defined?")
			return make([]string, 0)
		}

		taskNames := make([]string, len(tasksMap))

		i := 0
		for k := range tasksMap {
			taskNames[i] = k
			i++
		}

		return taskNames
	}

	return make([]string, 0)
}
