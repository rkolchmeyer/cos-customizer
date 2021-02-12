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
	"os/exec"

	"github.com/GoogleCloudPlatform/cos-customizer/src/pkg/utils"
)

type systemdClient struct {
	systemctl string
}

func (sc *systemdClient) isActive(unit string) bool {
	return exec.Command(sc.systemctl, "is-active", unit).Run() == nil
}

func (sc *systemdClient) stop(unit string) error {
	if sc.isActive(unit) {
		log.Printf("%q is active, stopping...", unit)
		if err := utils.RunCommand([]string{sc.systemctl, "stop", unit}, "", nil); err != nil {
			return err
		}
		log.Printf("%q stopped", unit)
	} else {
		log.Printf("%q is not active, ignoring", unit)
	}
	return nil
}