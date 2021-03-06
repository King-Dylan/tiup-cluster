// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiup-cluster/pkg/ansible"
	"github.com/pingcap-incubator/tiup-cluster/pkg/cliutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup-cluster/pkg/utils"
	tiuputils "github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	var (
		ansibleDir        string
		inventoryFileName string
		rename            string
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an exist TiDB cluster from TiDB-Ansible",
		RunE: func(cmd *cobra.Command, args []string) error {
			// migrate cluster metadata from Ansible inventory
			clsName, clsMeta, err := ansible.ImportAnsible(ansibleDir, inventoryFileName, sshTimeout)
			if err != nil {
				return err
			}

			// Use current directory as ansibleDir by default
			if ansibleDir == "" {
				if ansibleDir, err = os.Getwd(); err != nil {
					return err
				}
			}

			// Rename the imported cluster
			if rename != "" {
				clsName = rename
			}
			if clsName == "" {
				return fmt.Errorf("cluster name should not be empty")
			}
			if tiuputils.IsExist(meta.ClusterPath(clsName, meta.MetaFileName)) {
				return errDeployNameDuplicate.
					New("Cluster name '%s' is duplicated", clsName).
					WithProperty(cliutil.SuggestionFromFormat("Please use --rename `NAME` to specify another name (You can use `tiup cluster list` to see all clusters)"))
			}

			// copy SSH key to TiOps profile directory
			if err = utils.CreateDir(meta.ClusterPath(clsName, "ssh")); err != nil {
				return err
			}
			srcKeyPathPriv := ansible.SSHKeyPath()
			srcKeyPathPub := srcKeyPathPriv + ".pub"
			dstKeyPathPriv := meta.ClusterPath(clsName, "ssh", "id_rsa")
			dstKeyPathPub := dstKeyPathPriv + ".pub"
			if err = utils.CopyFile(srcKeyPathPriv, dstKeyPathPriv); err != nil {
				return err
			}
			if err = utils.CopyFile(srcKeyPathPub, dstKeyPathPub); err != nil {
				return err
			}

			// copy config files form deployment servers
			if err = ansible.ImportConfig(clsName, clsMeta, sshTimeout); err != nil {
				return err
			}

			if err = meta.SaveClusterMeta(clsName, clsMeta); err != nil {
				return err
			}

			// TODO: move original TiDB-Ansible directory to a staged location
			backupDir := meta.ClusterPath(clsName, "ansible-backup")
			if err = utils.Move(ansibleDir, backupDir); err != nil {
				return err
			}
			log.Infof("Ansible inventory saved in %s.", backupDir)

			log.Infof("Cluster %s imported.", clsName)
			fmt.Printf("Try `%s` to see the cluster.\n",
				color.HiYellowString("%s display %s", cmd.Parent().Use, clsName))
			return nil
		},
	}

	cmd.Flags().StringVarP(&ansibleDir, "dir", "d", "", "The path to TiDB-Ansible directory")
	cmd.Flags().StringVar(&inventoryFileName, "inventory", ansible.AnsibleInventoryFile, "The name of inventory file")
	cmd.Flags().StringVarP(&rename, "rename", "r", "", "Rename the imported cluster to `NAME`")

	return cmd
}
