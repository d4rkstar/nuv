// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package auth

import (
	"flag"
	"fmt"

	"github.com/zalando/go-keyring"
)

const usage = `Usage:
nuv login <apihost> [<user>]`

const whiskLoginPath = "/api/v1/web/whisk-system/login"
const defaultUser = "nuvolaris"
const nuvSecretServiceName = "nuvolaris"

func LoginCmd(args []string) error {
	flag.Usage = func() {
		fmt.Println(usage)
	}

	if len(args) == 0 {
		flag.Usage()
		return nil
	}

	fmt.Print("Enter Password: ")
	_, err := AskPassword()
	if err != nil {
		fmt.Println()
		return err
	}
	// url := args[0] + whiskLoginPath
	// user := defaultUser
	// if len(args) > 2 {
	// 	user = args[1]
	// }

	return nil
}

func storeCredentials(creds map[string]string) error {
	for k, v := range creds {
		err := keyring.Set(nuvSecretServiceName, k, v)
		if err != nil {
			return err
		}
	}

	return nil
}
