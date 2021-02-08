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

// provisioner is a tool for provisioning COS instances. The tool is intended to
// run on a running COS machine.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/google/subcommands"
)

var (
	stateDir = flag.String("state-dir", "/var/lib/.cos-customizer", "Absolute path to the directory to use for provisioner state. "+
		"This directory is used for persisting internal state across reboots, unpacking inputs, and running provisioning scripts. "+
		"The size of the directory scales with the size of the inputs.")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(&Run{}, "")
	flag.Parse()
	ctx := context.Background()
	ret := int(subcommands.Execute(ctx))
	os.Exit(ret)
}
