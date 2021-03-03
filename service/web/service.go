// The MIT License
//
// Copyright (c) 2020 Temporal Technologies Inc.  All rights reserved.
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package web

import (
	"fmt"
	"sync/atomic"

	webServer "github.com/temporalio/web-go/server"
	"go.temporal.io/server/common"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/persistence"
	persistenceClient "go.temporal.io/server/common/persistence/client"
	espersistence "go.temporal.io/server/common/persistence/elasticsearch"
	"go.temporal.io/server/common/resource"
	"go.temporal.io/server/common/searchattribute"
	"go.temporal.io/server/common/service/config"
	"go.temporal.io/server/common/service/dynamicconfig"
)

// Config represents configuration for frontend service
type Config struct {
	ESVisibilityListMaxQPS     dynamicconfig.IntPropertyFnWithNamespaceFilter
	ESIndexMaxResultWindow     dynamicconfig.IntPropertyFn
	PersistenceMaxQPS          dynamicconfig.IntPropertyFn
	PersistenceGlobalMaxQPS    dynamicconfig.IntPropertyFn
	EnableReadVisibilityFromES dynamicconfig.BoolPropertyFnWithNamespaceFilter
	ValidSearchAttributes      dynamicconfig.MapPropertyFn
	ThrottledLogRPS            dynamicconfig.IntPropertyFn
}

// NewConfig returns new service config with default values
func NewConfig(dc *dynamicconfig.Collection) *Config {
	return &Config{
		ESVisibilityListMaxQPS:     dc.GetIntPropertyFilteredByNamespace(dynamicconfig.FrontendESVisibilityListMaxQPS, 10),
		EnableReadVisibilityFromES: dc.GetBoolPropertyFnWithNamespaceFilter(dynamicconfig.EnableReadVisibilityFromES, false),
		ESIndexMaxResultWindow:     dc.GetIntProperty(dynamicconfig.FrontendESIndexMaxResultWindow, 10000),
		PersistenceMaxQPS:          dc.GetIntProperty(dynamicconfig.FrontendPersistenceMaxQPS, 2000),
		ValidSearchAttributes:      dc.GetMapProperty(dynamicconfig.ValidSearchAttributes, searchattribute.GetDefaultTypeMap()),
		PersistenceGlobalMaxQPS:    dc.GetIntProperty(dynamicconfig.FrontendPersistenceGlobalMaxQPS, 0),
		ThrottledLogRPS:            dc.GetIntProperty(dynamicconfig.FrontendThrottledLogRPS, 20),
	}
}

// Service represents the web ui service
type Service struct {
	resource.Resource

	status int32
	config *Config

	server *webServer.Server
}

// NewService builds a new web ui service
func NewService(params *resource.BootstrapParams) (resource.Resource, error) {
	serviceConfig := NewConfig(dynamicconfig.NewCollection(params.DynamicConfig, params.Logger))

	visibilityManagerInitializer := func(
		persistenceBean persistenceClient.Bean,
		logger log.Logger,
	) (persistence.VisibilityManager, error) {
		visibilityFromDB := persistenceBean.GetVisibilityManager()

		var visibilityFromES persistence.VisibilityManager
		if params.ESConfig != nil {
			visibilityIndexName := params.ESConfig.Indices[common.VisibilityAppName]
			visibilityConfigForES := &config.VisibilityConfig{
				MaxQPS:                 serviceConfig.PersistenceMaxQPS,
				VisibilityListMaxQPS:   serviceConfig.ESVisibilityListMaxQPS,
				ESIndexMaxResultWindow: serviceConfig.ESIndexMaxResultWindow,
				ValidSearchAttributes:  serviceConfig.ValidSearchAttributes,
			}
			visibilityFromES = espersistence.NewESVisibilityManager(visibilityIndexName, params.ESClient, visibilityConfigForES,
				nil, nil, params.MetricsClient, logger)
		}
		return persistence.NewVisibilityManagerWrapper(
			visibilityFromDB,
			visibilityFromES,
			serviceConfig.EnableReadVisibilityFromES,
			dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOff), // frontend visibility never write
		), nil
	}

	serviceResource, err := resource.New(
		params,
		common.WebUIServiceName,
		serviceConfig.PersistenceMaxQPS,
		serviceConfig.PersistenceGlobalMaxQPS,
		serviceConfig.ThrottledLogRPS,
		visibilityManagerInitializer,
	)
	if err != nil {
		return nil, err
	}

	return &Service{
		Resource: serviceResource,
		status:   common.DaemonStatusInitialized,
		config:   serviceConfig,
	}, nil
}

// Start starts the service
func (s *Service) Start() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	logger := s.GetLogger()
	logger.Info("Web UI starting")

	s.server = webServer.NewServer()

	if err := s.server.Start(); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to start Web UI server: %v.", err))
	}
}

// Stop stops the service
func (s *Service) Stop() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	s.server.Stop()
}
