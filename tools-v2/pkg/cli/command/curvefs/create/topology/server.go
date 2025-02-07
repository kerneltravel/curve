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
 * Created Date: 2022-06-30
 * Author: chengyi (Cyber-SiKu)
 */

package topology

import (
	"context"

	cmderror "github.com/opencurve/curve/tools-v2/internal/error"
	basecmd "github.com/opencurve/curve/tools-v2/pkg/cli/command"
	"github.com/opencurve/curve/tools-v2/proto/curvefs/proto/topology"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
)

type Server struct {
	Name         string `json:"name"`
	InternalIp   string `json:"internalip"`
	InternalPort uint32 `json:"internalport"`
	ExternalIp   string `json:"externalip"`
	ExternalPort uint32 `json:"externalport"`
	ZoneName     string `json:"zone"`
	PoolName     string `json:"pool"`
}

const (
	TYPE_SERVER = "server"
)

type DeleteServerRpc struct {
	Info           *basecmd.Rpc
	Request        *topology.DeleteServerRequest
	topologyClient topology.TopologyServiceClient
}

func (dsRpc *DeleteServerRpc) NewRpcClient(cc grpc.ClientConnInterface) {
	dsRpc.topologyClient = topology.NewTopologyServiceClient(cc)
}

func (dsRpc *DeleteServerRpc) Stub_Func(ctx context.Context) (interface{}, error) {
	return dsRpc.topologyClient.DeleteServer(ctx, dsRpc.Request)
}

var _ basecmd.RpcFunc = (*DeleteServerRpc)(nil) // check interface

type CreateServerRpc struct {
	Info           *basecmd.Rpc
	Request        *topology.ServerRegistRequest
	topologyClient topology.TopologyServiceClient
}

func (csRpc *CreateServerRpc) NewRpcClient(cc grpc.ClientConnInterface) {
	csRpc.topologyClient = topology.NewTopologyServiceClient(cc)
}

func (csRpc *CreateServerRpc) Stub_Func(ctx context.Context) (interface{}, error) {
	return csRpc.topologyClient.RegistServer(ctx, csRpc.Request)
}

var _ basecmd.RpcFunc = (*CreateServerRpc)(nil) // check interface

type ListZoneServerRpc struct {
	Info           *basecmd.Rpc
	Request        *topology.ListZoneServerRequest
	topologyClient topology.TopologyServiceClient
}

func (lzsRpc *ListZoneServerRpc) NewRpcClient(cc grpc.ClientConnInterface) {
	lzsRpc.topologyClient = topology.NewTopologyServiceClient(cc)
}

func (lzsRpc *ListZoneServerRpc) Stub_Func(ctx context.Context) (interface{}, error) {
	return lzsRpc.topologyClient.ListZoneServer(ctx, lzsRpc.Request)
}

var _ basecmd.RpcFunc = (*ListZoneServerRpc)(nil) // check interface

func (tCmd *TopologyCommand) listZoneServer(zoneId uint32) (*topology.ListZoneServerResponse, *cmderror.CmdError) {
	request := &topology.ListZoneServerRequest{
		ZoneID: &zoneId,
	}
	tCmd.listZoneServerRpc = &ListZoneServerRpc{
		Request: request,
	}
	tCmd.listZoneServerRpc.Info = basecmd.NewRpc(tCmd.addrs, tCmd.timeout, tCmd.retryTimes, "ListPoolZone")
	result, err := basecmd.GetRpcResponse(tCmd.listZoneServerRpc.Info, tCmd.listZoneServerRpc)
	if err.TypeCode() != cmderror.CODE_SUCCESS {
		return nil, err
	}
	response := result.(*topology.ListZoneServerResponse)
	return response, cmderror.ErrSuccess()
}

func (tCmd *TopologyCommand) scanServers() *cmderror.CmdError {
	// scan server
	for _, zone := range tCmd.clusterZonesInfo {
		response, err := tCmd.listZoneServer(zone.GetZoneID())
		if err.TypeCode() != cmderror.CODE_SUCCESS {
			return err
		}
		if response.GetStatusCode() != topology.TopoStatusCode_TOPO_OK {
			return cmderror.ErrListPool(response.GetStatusCode())
		}
		tCmd.clusterServersInfo = append(tCmd.clusterServersInfo, response.GetServerInfo()...)
	}
	// update delete server
	compare := func(server Server, serverInfo *topology.ServerInfo) bool {
		return server.Name == serverInfo.GetHostName() && server.ZoneName == serverInfo.GetZoneName() && server.PoolName == serverInfo.GetPoolName()
	}
	for _, serverInfo := range tCmd.clusterServersInfo {
		index := slices.IndexFunc(tCmd.topology.Servers, func(server Server) bool {
			return compare(server, serverInfo)
		})
		if index == -1 {
			id := serverInfo.GetServerID()
			request := &topology.DeleteServerRequest{
				ServerID: &id,
			}
			tCmd.deleteServer = append(tCmd.deleteServer, request)
			row := make(map[string]string)
			row[ROW_NAME] = serverInfo.GetHostName()
			row[ROW_TYPE] = TYPE_SERVER
			row[ROW_OPERATION] = ROW_VALUE_DEL
			row[ROW_PARENT] = serverInfo.GetZoneName()
			tCmd.Table.AddRow(row)
		}
	}

	// update create server
	for _, server := range tCmd.topology.Servers {
		index := slices.IndexFunc(tCmd.clusterServersInfo, func(serverInfo *topology.ServerInfo) bool {
			return compare(server, serverInfo)
		})
		if index == -1 {
			request := &topology.ServerRegistRequest{
				HostName:     &server.Name,
				InternalIp:   &server.InternalIp,
				InternalPort: &server.InternalPort,
				ExternalIp:   &server.ExternalIp,
				ExternalPort: &server.ExternalPort,
				ZoneName:     &server.ZoneName,
				PoolName:     &server.PoolName,
			}
			tCmd.createServer = append(tCmd.createServer, request)
			row := make(map[string]string)
			row[ROW_NAME] = server.Name
			row[ROW_TYPE] = TYPE_SERVER
			row[ROW_OPERATION] = ROW_VALUE_ADD
			row[ROW_PARENT] = server.ZoneName
			tCmd.Table.AddRow(row)
		}
	}

	return cmderror.ErrSuccess()
}

func (tCmd *TopologyCommand) removeServers() *cmderror.CmdError {
	tCmd.deleteServerRpc = &DeleteServerRpc{}
	tCmd.deleteServerRpc.Info = basecmd.NewRpc(tCmd.addrs, tCmd.timeout, tCmd.retryTimes, "DeleteServer")
	for _, delReuest := range tCmd.deleteServer {
		tCmd.deleteServerRpc.Request = delReuest
		result, err := basecmd.GetRpcResponse(tCmd.deleteServerRpc.Info, tCmd.deleteServerRpc)
		if err.TypeCode() != cmderror.CODE_SUCCESS {
			return err
		}
		response := result.(*topology.DeleteServerResponse)
		if response.GetStatusCode() != topology.TopoStatusCode_TOPO_OK {
			return cmderror.ErrDeleteTopology(response.GetStatusCode(), TYPE_SERVER)
		}
	}
	return cmderror.ErrSuccess()
}

func (tCmd *TopologyCommand) createServers() *cmderror.CmdError {
	tCmd.createServerRpc = &CreateServerRpc{}
	tCmd.createServerRpc.Info = basecmd.NewRpc(tCmd.addrs, tCmd.timeout, tCmd.retryTimes, "RegisterServer")
	for _, crtReuest := range tCmd.createServer {
		tCmd.createServerRpc.Request = crtReuest
		result, err := basecmd.GetRpcResponse(tCmd.createServerRpc.Info, tCmd.createServerRpc)
		if err.TypeCode() != cmderror.CODE_SUCCESS {
			return err
		}
		response := result.(*topology.ServerRegistResponse)
		if response.GetStatusCode() != topology.TopoStatusCode_TOPO_OK {
			return cmderror.ErrCreateTopology(response.GetStatusCode(), TYPE_SERVER)
		}
	}
	return cmderror.ErrSuccess()
}
