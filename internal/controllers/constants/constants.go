// Copyright The Cryostat Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package constants

import (
	certMeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	AuthProxyHttpContainerPort int32  = 4180
	CryostatHTTPContainerPort  int32  = 8181
	GrafanaContainerPort       int32  = 3000
	DatasourceContainerPort    int32  = 8989
	ReportsContainerPort       int32  = 10000
	StoragePort                int32  = 8333
	DatabasePort               int32  = 5432
	AgentProxyContainerPort    int32  = 8282
	AgentProxyHealthPort       int32  = 8281
	LoopbackAddress            string = "127.0.0.1"
	OperatorNamePrefix         string = "cryostat-operator-"
	OperatorDeploymentName     string = "cryostat-operator-controller"
	HttpPortName               string = "http"
	// CAKey is the key for a CA certificate within a TLS secret
	CAKey = certMeta.TLSCAKey
	// ALL capability to drop for restricted pod security. See:
	// https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
	CapabilityAll corev1.Capability = "ALL"

	// DatabaseSecretConnectionKey indexes the database connection password within the Cryostat database Secret
	DatabaseSecretConnectionKey = "CONNECTION_KEY"
	// DatabaseSecretEncryptionKey indexes the database encryption key within the Cryostat database Secret
	DatabaseSecretEncryptionKey = "ENCRYPTION_KEY"

	AgentProxyConfigFilePath string = "/etc/nginx-cryostat"
	AgentProxyConfigFileName string = "nginx.conf"

	targetNamespaceCRLabelPrefix    = "operator.cryostat.io/"
	TargetNamespaceCRNameLabel      = targetNamespaceCRLabelPrefix + "name"
	TargetNamespaceCRNamespaceLabel = targetNamespaceCRLabelPrefix + "namespace"

	CryostatCATLSCommonName     = "cryostat-ca-cert-manager"
	CryostatTLSCommonName       = "cryostat"
	DatabaseTLSCommonName       = "cryostat-db"
	StorageTLSCommonName        = "cryostat-storage"
	ReportsTLSCommonName        = "cryostat-reports"
	AgentsTLSCommonName         = "cryostat-agent"
	AgentAuthProxyTLSCommonName = "cryostat-agent-proxy"
)
