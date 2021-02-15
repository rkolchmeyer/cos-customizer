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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoogleCloudPlatform/cos-customizer/src/pkg/utils"
)

type installGPUStep struct {
	NvidiaDriverVersion      string
	NvidiaDriverMD5Sum       string
	NvidiaInstallDirHost     string
	NvidiaInstallerContainer string
	GCSDepsPrefix            string
}

func (s *installGPUStep) validate() error {
	if s.NvidiaDriverVersion == "" {
		return errors.New("invalid args: NvidiaDriverVersion is required in InstallGPU")
	}
	if s.NvidiaInstallerContainer == "" {
		return errors.New("invalid args: NvidiaInstallerContainer is required in InstallGPU")
	}
	return nil
}

func (s *installGPUStep) setDefaults() {
	if s.NvidiaInstallDirHost == "" {
		s.NvidiaInstallDirHost = "/var/lib/nvidia"
	}
}

func (s *installGPUStep) setupInstallDir(mountCmd string, dockerClient *docker) error {
	if err := os.MkdirAll(s.NvidiaInstallDirHost, 0755); err != nil {
		return err
	}
	if err := utils.RunCommand([]string{mountCmd, "--bind", s.NvidiaInstallDirHost, s.NvidiaInstallDirHost}, "", nil); err != nil {
		return fmt.Errorf("error bind mounting %q: %v", s.NvidiaInstallDirHost, err)
	}
	if err := utils.RunCommand([]string{mountCmd, "-o", "remount,exec", s.NvidiaInstallDirHost}, "", nil); err != nil {
		return fmt.Errorf("error remounting %q as executable: %v", s.NvidiaInstallDirHost, err)
	}
	return nil
}

func (s *installGPUStep) runInstaller(dockerClient *docker, driverVersion string) error {
	var downloadURL string
	if s.GCSDepsPrefix != "" {
		downloadURL = "https://storage.googleapis.com/" + strings.TrimPrefix(s.GCSDepsPrefix, "gs://")
	}
	var gpuInstallerDownloadURL string
	if strings.HasSuffix(s.NvidiaDriverVersion, ".run") && downloadURL != "" {
		gpuInstallerDownloadURL = downloadURL + "/" + s.NvidiaDriverVersion
	}
	log.Println("Running GPU installer...")
	if err := dockerClient.run([]string{
		"--rm",
		"--privileged",
		"--net=host",
		"--pid=host",
		"--volume", s.NvidiaInstallDirHost + ":/usr/local/nvidia",
		"--volume", "/dev:/dev",
		"--volume", "/:/root",
		"-e", "NVIDIA_DRIVER_VERSION",
		"-e", "NVIDIA_DRIVER_MD5SUM",
		"-e", "NVIDIA_INSTALL_DIR_HOST",
		"-e", "COS_NVIDIA_INSTALLER_CONTAINER",
		"-e", "NVIDIA_INSTALL_DIR_CONTAINER",
		"-e", "ROOT_MOUNT_DIR",
		"-e", "COS_DOWNLOAD_GCS",
		"-e", "GPU_INSTALLER_DOWNLOAD_URL",
		s.NvidiaInstallerContainer,
	}, []string{
		"NVIDIA_DRIVER_VERSION=" + driverVersion,
		"NVIDIA_DRIVER_MD5SUM=" + s.NvidiaDriverMD5Sum,
		"NVIDIA_INSTALL_DIR_HOST=" + s.NvidiaInstallDirHost,
		"COS_NVIDIA_INSTALLER_CONTAINER=" + s.NvidiaInstallerContainer,
		"NVIDIA_INSTALL_DIR_CONTAINER=/usr/local/nvidia",
		"ROOT_MOUNT_DIR=/root",
		"COS_DOWNLOAD_GCS=" + downloadURL,
		"GPU_INSTALLER_DOWNLOAD_URL=" + gpuInstallerDownloadURL,
	}); err != nil {
		log.Printf("GPU install failed.")
		logFile, openErr := os.Open(filepath.Join(s.NvidiaInstallDirHost, "nvidia-installer.log"))
		if openErr != nil {
			log.Printf("Cannot open GPU installer log file: %v", openErr)
			return err
		}
		defer logFile.Close()
		log.Println("Dumping GPU installer logs to stdout")
		if _, copyErr := io.Copy(os.Stdout, logFile); copyErr != nil {
			log.Printf("Cannot dump GPU installer logs: %v", copyErr)
			return err
		}
		return err
	}
	log.Println("Done running GPU installer")
	return nil
}

func processExists(rootDir, name string) (bool, error) {
	files, err := filepath.Glob(filepath.Join(rootDir, "proc", "*", "cmdline"))
	if err != nil {
		return false, err
	}
	for _, file := range files {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return false, err
		}
		for i := range data {
			if data[i] == 0x00 {
				data[i] = 0x20
			}
		}
		if strings.Contains(string(data), name) {
			return true, nil
		}
	}
	return false, nil
}

func (s *installGPUStep) run(runState *state, deps Deps) error {
	if err := s.validate(); err != nil {
		return err
	}
	s.setDefaults()
	dockerClient := &docker{
		dockerCmd:  deps.DockerCmd,
		journalctl: deps.JournalctlCmd,
	}
	var driverVersion string
	if strings.HasSuffix(s.NvidiaDriverVersion, ".run") {
		// NVIDIA-Linux-x86_64-450.51.06.run -> 450.51.06
		fields := strings.FieldsFunc(strings.TrimSuffix(s.NvidiaDriverVersion, ".run"), func(r rune) bool { return r == '-' })
		if len(fields) < 4 {
			return fmt.Errorf("Malformed nvidia installer: %q", s.NvidiaDriverVersion)
		}
		driverVersion = fields[3]
	} else {
		driverVersion = s.NvidiaDriverVersion
	}
	log.Println("Installing GPU drivers...")
	if err := s.setupInstallDir(deps.MountCmd, dockerClient); err != nil {
		return err
	}
	if err := dockerClient.pull([]string{s.NvidiaInstallerContainer}); err != nil {
		return err
	}
	if err := s.runInstaller(dockerClient, driverVersion); err != nil {
		return err
	}
	// Run nvidia-smi to sanity check the installation
	if err := utils.RunCommand([]string{filepath.Join(s.NvidiaInstallDirHost, "bin", "nvidia-smi")}, "", nil); err != nil {
		return err
	}
	// Start nvidia-persistenced
	running, err := processExists(deps.RootDir, "nvidia-persistenced")
	if err != nil {
		return fmt.Errorf(`error searching for process "nvidia-persistenced": %v`, err)
	}
	if !running {
		log.Println("nvidia-persistenced is not running: starting nvidia-persistenced")
		if err := utils.RunCommand([]string{filepath.Join(s.NvidiaInstallDirHost, "bin", "nvidia-persistenced"), "--verbose"}, "", nil); err != nil {
			return err
		}
	} else {
		log.Println("nvidia-persistenced is already running")
	}
	// Set softlockup_panic
	if err := ioutil.WriteFile(filepath.Join(deps.RootDir, "proc", "sys", "kernel", "softlockup_panic"), []byte("1"), 0644); err != nil {
		return err
	}
	log.Println("Done installing GPU drivers")
	return nil
}
