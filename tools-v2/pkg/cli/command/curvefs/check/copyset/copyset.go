/*
 *  Copyright (c) 2022 NetEase Inc.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

/*
 * Project: CurveCli
 * Created Date: 2022-06-22
 * Author: chengyi (Cyber-SiKu)
 */

package copyset

import (
	"fmt"
	"sort"

	"github.com/liushuochen/gotable"
	"github.com/liushuochen/gotable/table"
	cmderror "github.com/opencurve/curve/tools-v2/internal/error"
	cobrautil "github.com/opencurve/curve/tools-v2/internal/utils"
	basecmd "github.com/opencurve/curve/tools-v2/pkg/cli/command"
	"github.com/opencurve/curve/tools-v2/pkg/cli/command/curvefs/query/copyset"
	"github.com/opencurve/curve/tools-v2/pkg/config"
	"github.com/opencurve/curve/tools-v2/pkg/output"
	"github.com/spf13/cobra"
)

const (
	ROW_COPYSETKEY = "copyset key"
	ROW_STATUS     = "status"
	ROW_EXPLAIN    = "explain"
)

type CopysetCommand struct {
	basecmd.FinalCurveCmd
	key2Copyset       *map[uint64]*cobrautil.CopysetInfoStatus
	copysetKey2Status *map[uint64]cobrautil.COPYSET_HEALTH_STATUS
}

var _ basecmd.FinalCurveCmdFunc = (*CopysetCommand)(nil) // check interface

func NewCopysetCommand() *cobra.Command {
	copysetCmd := &CopysetCommand{
		FinalCurveCmd: basecmd.FinalCurveCmd{
			Use:   "copyset",
			Short: "check copysets health in curvefs",
		},
	}
	basecmd.NewFinalCurveCli(&copysetCmd.FinalCurveCmd, copysetCmd)
	return copysetCmd.Cmd
}

func NewCheckCopysetCommand() *CopysetCommand {
	copysetCmd := &CopysetCommand{
		FinalCurveCmd: basecmd.FinalCurveCmd{
			Use:   "copyset",
			Short: "check copysets health in curvefs",
		},
	}
	basecmd.NewFinalCurveCli(&copysetCmd.FinalCurveCmd, copysetCmd)
	return copysetCmd
}

func GetCopysetsStatus(caller *cobra.Command, copysetIds string, poolIds string) (interface{}, *table.Table, *cmderror.CmdError) {
	checkCopyset := NewCheckCopysetCommand()
	checkCopyset.Cmd.SetArgs([]string{
		fmt.Sprintf("--%s", config.CURVEFS_COPYSETID), copysetIds,
		fmt.Sprintf("--%s", config.CURVEFS_POOLID), poolIds,
		fmt.Sprintf("--%s", config.FORMAT), config.FORMAT_NOOUT,
	})
	cobrautil.AlignFlags(caller, checkCopyset.Cmd, []string{config.RPCRETRYTIMES, config.RPCTIMEOUT, config.CURVEFS_MDSADDR})
	checkCopyset.Cmd.SilenceUsage = true
	err := checkCopyset.Cmd.Execute()
	if err != nil {
		retErr := cmderror.ErrCheckCopyset()
		retErr.Format(err.Error())
		return checkCopyset.Result, checkCopyset.Table, retErr
	}
	return checkCopyset.Result, checkCopyset.Table, checkCopyset.Error
}

func (cCmd *CopysetCommand) AddFlags() {
	config.AddRpcRetryTimesFlag(cCmd.Cmd)
	config.AddRpcTimeoutFlag(cCmd.Cmd)
	config.AddFsMdsAddrFlag(cCmd.Cmd)
	config.AddCopysetidSliceRequiredFlag(cCmd.Cmd)
	config.AddPoolidSliceRequiredFlag(cCmd.Cmd)
}

func (cCmd *CopysetCommand) Init(cmd *cobra.Command, args []string) error {
	var queryCopysetErr *cmderror.CmdError
	cCmd.key2Copyset, queryCopysetErr = copyset.QueryCopysetInfoStatus(cCmd.Cmd)
	if queryCopysetErr.TypeCode() != cmderror.CODE_SUCCESS {
		return queryCopysetErr.ToError()
	}
	cCmd.Error = queryCopysetErr
	table, err := gotable.Create(ROW_COPYSETKEY, ROW_STATUS, ROW_EXPLAIN)
	if err != nil {
		return err
	}
	cCmd.Table = table
	copysetKey2Status := make(map[uint64]cobrautil.COPYSET_HEALTH_STATUS)
	cCmd.copysetKey2Status = &copysetKey2Status
	return nil
}

func (cCmd *CopysetCommand) Print(cmd *cobra.Command, args []string) error {
	return output.FinalCmdOutput(&cCmd.FinalCurveCmd, cCmd)
}

func (cCmd *CopysetCommand) RunCommand(cmd *cobra.Command, args []string) error {
	rows := make([]map[string]string, 0)
	var errs []*cmderror.CmdError
	for k, v := range *cCmd.key2Copyset {
		row := make(map[string]string)
		row[ROW_COPYSETKEY] = fmt.Sprintf("%d", k)
		if v == nil {
			row[ROW_STATUS] = cobrautil.CopysetHealthStatus_Str[int32(cobrautil.COPYSET_NOTEXIST)]
		} else {
			status, errsCheck := cobrautil.CheckCopySetHealth(v)
			row[ROW_STATUS] = cobrautil.CopysetHealthStatus_Str[int32(status)]
			if status != cobrautil.COPYSET_OK {
				explain := "|"
				for _, e := range errsCheck {
					explain += fmt.Sprintf("%s|", e.Message)
					errs = append(errs, e)
				}
				row[ROW_EXPLAIN] = explain
			}
		}
		rows = append(rows, row)
	}
	retErr := cmderror.MergeCmdError(errs)
	cCmd.Error = &retErr
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][ROW_COPYSETKEY] < rows[j][ROW_COPYSETKEY]
	})
	cCmd.Table.AddRows(rows)
	var err error
	cCmd.Result, err = cobrautil.TableToResult(cCmd.Table)
	return err
}

func (cCmd *CopysetCommand) ResultPlainOutput() error {
	return output.FinalCmdOutputPlain(&cCmd.FinalCurveCmd, cCmd)
}
