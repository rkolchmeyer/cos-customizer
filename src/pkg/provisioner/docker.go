// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provisioner

import (
	"log"
	"os"

	"github.com/GoogleCloudPlatform/cos-customizer/src/pkg/utils"
)

type docker struct {
	dockerCmd  string
	journalctl string
}

func (d *docker) run(args []string, env []string) error {
	cmd := []string{d.dockerCmd, "run"}
	cmd = append(cmd, args...)
	return utils.RunCommand(cmd, "", append(os.Environ(), env...))
}

func (d *docker) pull(args []string) error {
	var err error
	cmd := []string{d.dockerCmd, "pull"}
	cmd = append(cmd, args...)
	// `docker pull` can sometimes flake if the VM's network is still being
	// initialized. Retry a few times.
	for i := 1; i <= 10; i++ {
		log.Printf("Running command %v... [%d/10]", cmd, i)
		err = utils.RunCommand(cmd, "", nil)
		if err == nil {
			log.Printf("Successfully ran command %v", cmd)
			return nil
		}
	}
	log.Printf("Command %v failed. See stdout for journal logs.", cmd)
	if err := utils.RunCommand([]string{d.journalctl, "-u", "docker.service", "--no-pager"}, "", nil); err != nil {
		log.Println(err)
	}
	return err
}
