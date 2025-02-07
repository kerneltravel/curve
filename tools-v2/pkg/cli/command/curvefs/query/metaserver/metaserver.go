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
 * Created Date: 2022-06-16
 * Author: chengyi (Cyber-SiKu)
 */

package metaserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/liushuochen/gotable"
	cmderror "github.com/opencurve/curve/tools-v2/internal/error"
	basecmd "github.com/opencurve/curve/tools-v2/pkg/cli/command"
	"github.com/opencurve/curve/tools-v2/pkg/config"
	"github.com/opencurve/curve/tools-v2/pkg/output"
	"github.com/opencurve/curve/tools-v2/proto/curvefs/proto/topology"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

type QueryMetaserverRpc struct {
	Info           *basecmd.Rpc
	Request        *topology.GetMetaServerInfoRequest
	topologyClient topology.TopologyServiceClient
}

var _ basecmd.RpcFunc = (*QueryMetaserverRpc)(nil) // check interface

type MetaserverCommand struct {
	basecmd.FinalCurveCmd
	Rpc  []*QueryMetaserverRpc
	Rows []map[string]string
}

var _ basecmd.FinalCurveCmdFunc = (*MetaserverCommand)(nil) // check interface

func (qmRpc *QueryMetaserverRpc) NewRpcClient(cc grpc.ClientConnInterface) {
	qmRpc.topologyClient = topology.NewTopologyServiceClient(cc)
}

func (qmRpc *QueryMetaserverRpc) Stub_Func(ctx context.Context) (interface{}, error) {
	return qmRpc.topologyClient.GetMetaServer(ctx, qmRpc.Request)
}

func NewMetaserverCommand() *cobra.Command {
	metaserverCmd := &MetaserverCommand{
		FinalCurveCmd: basecmd.FinalCurveCmd{
			Use:   "metaserver",
			Short: "query metaserver in curvefs by metaserverid or metaserveraddr",
			Long:  "when both metaserverid and metaserveraddr exist, query only by metaserverid",
		},
	}
	basecmd.NewFinalCurveCli(&metaserverCmd.FinalCurveCmd, metaserverCmd)
	return metaserverCmd.Cmd
}

func (mCmd *MetaserverCommand) AddFlags() {
	config.AddRpcRetryTimesFlag(mCmd.Cmd)
	config.AddRpcTimeoutFlag(mCmd.Cmd)
	config.AddFsMdsAddrFlag(mCmd.Cmd)
	config.AddMetaserverAddrOptionFlag(mCmd.Cmd)
	config.AddMetaserverIdOptionFlag(mCmd.Cmd)
}

func (mCmd *MetaserverCommand) Init(cmd *cobra.Command, args []string) error {
	addrs, addrErr := config.GetFsMdsAddrSlice(mCmd.Cmd)
	if addrErr.TypeCode() != cmderror.CODE_SUCCESS {
		return fmt.Errorf(addrErr.Message)
	}

	var metaserverAddrs []string
	var metaserverIds []string
	if viper.IsSet(config.VIPER_CURVEFS_METASERVERADDR) && !viper.IsSet(config.VIPER_CURVEFS_METASERVERID) {
		// metaserveraddr is set, but metaserverid is not set
		metaserverAddrs = viper.GetStringSlice(config.VIPER_CURVEFS_METASERVERADDR)
	} else {
		metaserverIds = viper.GetStringSlice(config.VIPER_CURVEFS_METASERVERID)
	}

	if len(metaserverAddrs) == 0 && len(metaserverIds) == 0 {
		return fmt.Errorf("%s or %s is required", config.CURVEFS_METASERVERADDR, config.CURVEFS_METASERVERID)
	}

	table, err := gotable.Create("id", "host name", "internal addr", "external addr", "online state")
	if err != nil {
		return err
	}

	mCmd.Table = table

	mCmd.Rows = make([]map[string]string, 0)
	timeout := viper.GetDuration(config.VIPER_GLOBALE_RPCTIMEOUT)
	retrytimes := viper.GetInt32(config.VIPER_GLOBALE_RPCRETRYTIMES)
	for i := range metaserverAddrs {
		addr := strings.Split(metaserverAddrs[i], ":")
		if len(addr) != 2 {
			return fmt.Errorf("unrecognized metaserver addr: %s", metaserverAddrs[i])
		}
		port, err := strconv.ParseUint(addr[1], 10, 32)
		if err != nil {
			return fmt.Errorf("unrecognized metaserver port: %s", metaserverAddrs[i])
		}
		port32 := uint32(port)
		request := &topology.GetMetaServerInfoRequest{
			HostIp: &addr[0],
			Port:   &port32,
		}
		rpc := &QueryMetaserverRpc{
			Request: request,
		}
		rpc.Info = basecmd.NewRpc(addrs, timeout, retrytimes, "GetMetaServerInfo")
		mCmd.Rpc = append(mCmd.Rpc, rpc)
		row := make(map[string]string)
		row["id"] = ""
		row["external addr"] = metaserverAddrs[i]
		mCmd.Rows = append(mCmd.Rows, row)
	}

	for i := range metaserverIds {
		id, err := strconv.ParseUint(metaserverIds[i], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid %s: %s", config.CURVEFS_METASERVERID, metaserverIds[i])
		}
		id32 := uint32(id)
		request := &topology.GetMetaServerInfoRequest{
			MetaServerID: &id32,
		}
		rpc := &QueryMetaserverRpc{
			Request: request,
		}
		rpc.Info = basecmd.NewRpc(addrs, timeout, retrytimes, "GetMetaServerInfo")
		mCmd.Rpc = append(mCmd.Rpc, rpc)
		row := make(map[string]string)
		row["id"] = metaserverIds[i]
		row["external addr"] = ""
		mCmd.Rows = append(mCmd.Rows, row)
	}

	return nil
}

func (mCmd *MetaserverCommand) Print(cmd *cobra.Command, args []string) error {
	return output.FinalCmdOutput(&mCmd.FinalCurveCmd, mCmd)
}

func (mCmd *MetaserverCommand) RunCommand(cmd *cobra.Command, args []string) error {
	var infos []*basecmd.Rpc
	var funcs []basecmd.RpcFunc
	for _, rpc := range mCmd.Rpc {
		infos = append(infos, rpc.Info)
		funcs = append(funcs, rpc)
	}

	results, err := basecmd.GetRpcListResponse(infos, funcs)
	var errs []*cmderror.CmdError
	if err.TypeCode() != cmderror.CODE_SUCCESS {
		errs = append(errs, err)
	}
	var resList []interface{}
	for _, result := range results {
		response := result.(*topology.GetMetaServerInfoResponse)
		res, err := output.MarshalProtoJson(response)
		if err != nil {
			errMar := cmderror.ErrMarShalProtoJson()
			errMar.Format(err.Error())
			errs = append(errs, errMar)
		}
		resList = append(resList, res)
		if response.GetStatusCode() != topology.TopoStatusCode_TOPO_OK {
			code := response.GetStatusCode()
			err := cmderror.ErrGetFsInfo(int(code))
			err.Format(topology.TopoStatusCode_name[int32(response.GetStatusCode())])
			errs = append(errs, err)
			continue
		}
		metaserverInfo := response.GetMetaServerInfo()
		for _, row := range mCmd.Rows {
			id := strconv.FormatUint(uint64(metaserverInfo.GetMetaServerID()), 10)
			externalAddr := fmt.Sprintf("%s:%d", metaserverInfo.GetExternalIp(), metaserverInfo.GetExternalPort())
			if row["id"] == id || row["external addr"] == externalAddr {
				row["id"] = id
				row["host name"] = metaserverInfo.GetHostname()
				internalAddr := fmt.Sprintf("%s:%d", metaserverInfo.GetInternalIp(), metaserverInfo.GetInternalPort())
				row["internal addr"] = internalAddr
				row["external addr"] = externalAddr
				row["online state"] = metaserverInfo.GetOnlineState().String()
			}
		}
	}

	mCmd.Table.AddRows(mCmd.Rows)
	mCmd.Result = resList
	mCmd.Error = cmderror.MostImportantCmdError(errs)

	return nil
}

func (mCmd *MetaserverCommand) ResultPlainOutput() error {
	return output.FinalCmdOutputPlain(&mCmd.FinalCurveCmd, mCmd)
}
