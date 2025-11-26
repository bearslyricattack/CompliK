// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package processor

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProcessAnalyzer", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "proc-analyzer-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("ReadProcessStatus", func() {
		It("should parse status file correctly", func() {
			// Create mock process directory
			pidDir := filepath.Join(tmpDir, "1234")
			err := os.Mkdir(pidDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Create mock status file
			statusContent := `Name:	nginx
Umask:	0022
State:	S (sleeping)
Tgid:	1234
Ngid:	0
Pid:	1234
PPid:	1
TracerPid:	0
Uid:	0	0	0	0
Gid:	0	0	0	0
FDSize:	64
Groups:
NStgid:	1234	1
NSpid:	1234	1
NSpgid:	1234	1
NSsid:	1234	1
VmPeak:	   12345 kB
VmSize:	   12345 kB`

			statusPath := filepath.Join(pidDir, "status")
			err = os.WriteFile(statusPath, []byte(statusContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			status, err := ReadProcessStatus(tmpDir, 1234)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).NotTo(BeNil())
			Expect(status.PID).To(Equal(1234))
			Expect(status.Name).To(Equal("nginx"))
			Expect(status.PPID).To(Equal(1))
			Expect(status.Tgid).To(Equal(1234))
			Expect(status.Tracer).To(Equal(0))
			Expect(status.NSpid).To(HaveLen(2))
			Expect(status.NSpid[0]).To(Equal(1234))
			Expect(status.NSpid[1]).To(Equal(1))
		})

		It("should handle container main process with NSpid", func() {
			pidDir := filepath.Join(tmpDir, "5678")
			err := os.Mkdir(pidDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Container main process has NSpid ending with 1
			statusContent := `Name:	app
Tgid:	5678
Pid:	5678
PPid:	5677
TracerPid:	0
NSpid:	5678	1`

			statusPath := filepath.Join(pidDir, "status")
			err = os.WriteFile(statusPath, []byte(statusContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			status, err := ReadProcessStatus(tmpDir, 5678)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.NSpid).To(HaveLen(2))
			Expect(status.NSpid[1]).To(Equal(1))
		})

		It("should return error when status file doesn't exist", func() {
			_, err := ReadProcessStatus(tmpDir, 9999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read status file"))
		})

		It("should handle minimal status file", func() {
			pidDir := filepath.Join(tmpDir, "1111")
			err := os.Mkdir(pidDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			statusContent := `Name:	test
Pid:	1111`

			statusPath := filepath.Join(pidDir, "status")
			err = os.WriteFile(statusPath, []byte(statusContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			status, err := ReadProcessStatus(tmpDir, 1111)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.Name).To(Equal("test"))
			Expect(status.PID).To(Equal(1111))
		})
	})

	Describe("IsContainerMainProcess", func() {
		It("should identify container main process", func() {
			status := &ProcessStatus{
				PID:   1234,
				PPID:  1,
				NSpid: []int{1234, 1}, // Last value is 1 = main process
			}
			Expect(IsContainerMainProcess(status)).To(BeTrue())
		})

		It("should identify non-main process in container", func() {
			status := &ProcessStatus{
				PID:   5678,
				PPID:  1234,
				NSpid: []int{5678, 42}, // Last value is not 1
			}
			Expect(IsContainerMainProcess(status)).To(BeFalse())
		})

		It("should return false for host process", func() {
			status := &ProcessStatus{
				PID:   1234,
				PPID:  1,
				NSpid: []int{1234}, // Only one namespace = host
			}
			Expect(IsContainerMainProcess(status)).To(BeFalse())
		})

		It("should handle nil status", func() {
			Expect(IsContainerMainProcess(nil)).To(BeFalse())
		})

		It("should handle empty NSpid", func() {
			status := &ProcessStatus{
				PID:   1234,
				NSpid: []int{},
			}
			Expect(IsContainerMainProcess(status)).To(BeFalse())
		})

		It("should handle multi-level namespaces", func() {
			status := &ProcessStatus{
				PID:   1234,
				NSpid: []int{1234, 500, 1}, // Nested containers, innermost is PID 1
			}
			Expect(IsContainerMainProcess(status)).To(BeTrue())
		})
	})

	Describe("FindContainerMainProcess", func() {
		It("should find main process when direct", func() {
			// Create main process
			mainPidDir := filepath.Join(tmpDir, "1000")
			err := os.Mkdir(mainPidDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			statusContent := `Name:	main
Pid:	1000
PPid:	999
NSpid:	1000	1`

			err = os.WriteFile(filepath.Join(mainPidDir, "status"), []byte(statusContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			mainPID, err := FindContainerMainProcess(tmpDir, 1000)
			Expect(err).NotTo(HaveOccurred())
			Expect(mainPID).To(Equal(1000))
		})

		It("should trace back to main process through parent", func() {
			// Create main process (PID 1000)
			mainPidDir := filepath.Join(tmpDir, "1000")
			os.Mkdir(mainPidDir, 0755)
			mainStatus := `Name:	main
Pid:	1000
PPid:	999
NSpid:	1000	1`
			os.WriteFile(filepath.Join(mainPidDir, "status"), []byte(mainStatus), 0644)

			// Create child process (PID 1001)
			childPidDir := filepath.Join(tmpDir, "1001")
			os.Mkdir(childPidDir, 0755)
			childStatus := `Name:	child
Pid:	1001
PPid:	1000
NSpid:	1001	42`
			os.WriteFile(filepath.Join(childPidDir, "status"), []byte(childStatus), 0644)

			mainPID, err := FindContainerMainProcess(tmpDir, 1001)
			Expect(err).NotTo(HaveOccurred())
			Expect(mainPID).To(Equal(1000))
		})

		It("should handle multi-level parent chain", func() {
			// Create grandparent (main process)
			os.Mkdir(filepath.Join(tmpDir, "2000"), 0755)
			gpStatus := `Name:	grandparent
Pid:	2000
PPid:	1999
NSpid:	2000	1`
			os.WriteFile(filepath.Join(tmpDir, "2000", "status"), []byte(gpStatus), 0644)

			// Create parent
			os.Mkdir(filepath.Join(tmpDir, "2001"), 0755)
			pStatus := `Name:	parent
Pid:	2001
PPid:	2000
NSpid:	2001	10`
			os.WriteFile(filepath.Join(tmpDir, "2001", "status"), []byte(pStatus), 0644)

			// Create child
			os.Mkdir(filepath.Join(tmpDir, "2002"), 0755)
			cStatus := `Name:	child
Pid:	2002
PPid:	2001
NSpid:	2002	20`
			os.WriteFile(filepath.Join(tmpDir, "2002", "status"), []byte(cStatus), 0644)

			mainPID, err := FindContainerMainProcess(tmpDir, 2002)
			Expect(err).NotTo(HaveOccurred())
			Expect(mainPID).To(Equal(2000))
		})

		It("should return error when process doesn't exist", func() {
			_, err := FindContainerMainProcess(tmpDir, 9999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read process status"))
		})

		It("should return error when reaching init without finding main process", func() {
			// Create a process with PPID 0 (init) but not a container main process
			pidDir := filepath.Join(tmpDir, "3000")
			os.Mkdir(pidDir, 0755)
			statusContent := `Name:	init
Pid:	3000
PPid:	0
NSpid:	3000`
			os.WriteFile(filepath.Join(pidDir, "status"), []byte(statusContent), 0644)

			_, err := FindContainerMainProcess(tmpDir, 3000)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reached init process"))
		})

		It("should detect circular reference", func() {
			// Create two processes that reference each other (pathological case)
			os.Mkdir(filepath.Join(tmpDir, "4000"), 0755)
			status1 := `Name:	proc1
Pid:	4000
PPid:	4001
NSpid:	4000	10`
			os.WriteFile(filepath.Join(tmpDir, "4000", "status"), []byte(status1), 0644)

			os.Mkdir(filepath.Join(tmpDir, "4001"), 0755)
			status2 := `Name:	proc2
Pid:	4001
PPid:	4000
NSpid:	4001	11`
			os.WriteFile(filepath.Join(tmpDir, "4001", "status"), []byte(status2), 0644)

			_, err := FindContainerMainProcess(tmpDir, 4000)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("circular"))
		})
	})

	Describe("GetProcessNamespaceInfo", func() {
		It("should return error when namespace directory doesn't exist", func() {
			_, err := GetProcessNamespaceInfo(tmpDir, 9999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read namespace directory"))
		})

		It("should handle empty namespace directory", func() {
			pidDir := filepath.Join(tmpDir, "5000")
			nsDir := filepath.Join(pidDir, "ns")
			err := os.MkdirAll(nsDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			nsInfo, err := GetProcessNamespaceInfo(tmpDir, 5000)
			Expect(err).NotTo(HaveOccurred())
			Expect(nsInfo).To(BeEmpty())
		})
	})
})
