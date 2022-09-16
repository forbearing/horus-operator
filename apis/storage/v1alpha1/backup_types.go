/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSpec defines the desired state of Backup
type BackupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	Schedule string `json:"schedule"`

	// The number of backup to be retained. Value must be non-negative interger.
	// Default to 0, and means keep all backups.
	// +optional
	Retention uint64 `json:"retention"`

	// BackupFrom specifies where the data should be backup from
	// currently supported: pod, deployment, statefulset, daemonset,
	// persistentvolume and persistentvolumeclaim.
	// It's ignore case.
	BackupFrom *BackupFrom `json:"backupFrom"`

	// BackupTo specifies where the data shoud be backup to
	// currently supported: cephfs, nfs, persistentVolumeClaim,
	// S3, Minio, Server, RestServer
	BackupTo *BackupTo `json:"backupTo"`

	// backup timeout
	// +optional
	Timeout metav1.Duration `json:"timeout"`

	// Environment variable passed to backup program.
	// +optional
	Env []corev1.EnvVar `json:"env"`

	// TimeZone
	// +optional
	TimeZone string `json:"timezone"`

	// Cluster Name
	// +optional
	Cluster string `json:"cluster"`
}

// BackupFrom defines where the data should backup from
type BackupFrom struct {
	Name     string   `json:"name"`
	Resource Resource `json:"resource"`
}

type Resource string

const (
	PodResource           Resource = "pod"
	DeploymentResource    Resource = "deployment"
	StatefulSetResource   Resource = "statefulset"
	DaemonSetResource     Resource = "daemonset"
	PersistentVolume      Resource = "persistentvolume"
	PersistentVolumeClaim Resource = "persistentvolumeclaim"
)

// BackupTo defines where the data shoud be backup to
type BackupTo struct {
	// backup to nfs server
	// +optional
	NFS *NFS `json:"nfs"`
	// backup to PersistentVolumeClaim
	// +optional
	PVC *PVC `json:"pvc"`
	// backup to CephFS
	// +optional
	CephFS *CephFS `json:"cephfs"`
	// backup to S3
	// +optional
	S3 *S3 `json:"s3"`
	// backup to rest server
	// +optional
	RestServer *RestServer `json:"restServer"`
	// backup to sftp
	// +optional
	SFTP *SFTP `json:"sftp"`
	// backup to rclone
	// +optional
	Rclone *Rclone `json:"rclone"`
}

type NFS struct {
	// server is the hostname or IP address of the NFS server.
	Server string `json:"server"`
	// path that is exported by the NFS server.
	Path string `json:"path"`
}

type PVC struct {
	//// Name is this PersistentVolumeClaim name.
	//Name string `json:"name"`
	//// Namespace is this PersistentVolumeClaim namespace.
	//Namespace string `json:"namespace"`
	//// StorageClassName is the name of the StorageClass for which PVC claim PV.
	//StorageClassName string `json:"storageClassName"`
	//// VolumeName is the binding reference to the PersistentVolume backing this claim.
	//VolumeName string `json:"volumeName"`
	//// AccessModes contains the desired access modes the volume should have.
	//AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes"`
	corev1.PersistentVolumeClaim `json:"persistentVolumeClaim"`
}

type CephFS struct {
	Monitors []string `json:"monitors" `
	Path     string   `json:"path" `
	// secet.data should containe three field: user, keyring, clusterID
	CredentialName      string `json:"credentialName"`
	CredentialNamespace string `json:"credentialNamespace"`
}

type S3 struct {
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	Folder   string `json:"folder"`
	// secret.data should contain two field: accessKey, secretKey
	CredentialName        string `json:"credentialName"`
	CredentialNamespace   string `json:"credentialNamespace"`
	InsecureTLSSkipVerify bool   `json:"insecureTLSSkipVerify"`
	Region                string `json:"region"`
}

type RestServer struct {
	Address string `json:"address"`
	Port    int32  `json:"port"`
	Path    string `json:"path"`
	// secret.data should contain two field: username, password
	CredentialName      string `json:"credentialName"`
	CredentialNamespace string `json:"credentialNamespace"`
}

type SFTP struct {
	Address string `json:"address"`
	Port    int32  `json:"port"`
	Path    string `json:"path"`
	// secret.data should contain two field: username, password
	CredentialName      string `json:"credentialName"`
	CredentialNamespace string `json:"credentialNamespace"`
}

type Rclone struct {
	Address string `json:"address"`
	Path    string `json:"path"`
}

// BackupStatus defines the observed state of Backup
type BackupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Backup is the Schema for the backups API
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}

//// s3 example:
//// s3:
////     endpoint: s3.us-south-3.amazonaws.com
////     credentialSecretName: s3-creds
////     credentialSecretNamespace: default
////     bucketName: backups
////     folder: backups
////
//// minio example:
//// s3:
////     endpoint: minio.example.com
////     endpointCA: LS0tLS1CRUdVqeXplRFB6bFJycjlpbEpWaVZ1......
////     credentialSecretName: minio-creds
////     credentialSecretNamespace: default
////     bucketName: backups
////
//// credentialSecret example:
//// apiVersion: v1
//// kind: Secret
//// metadata:
////   name: my-creds
//// type: Opaque
//// data:
////   accessKey: <Enter your base64-encoded access key>
////   secretKey: <Enter your base64-encoded secret key>
//type S3 struct {
//    // The endpoint is used to access S3 in the region of your bucket.
//    Endpoint string `json:"endpoint"`
//    // This must be the base64 encoded CA cert.
//    EndpointCA string `json:"endpointCA"`
//    // You should always set to true if you are not using TLS.
//    InsecureTLSSkipVerify bool `json:"insecureTLSSkipVerify"`
//    // If you need to use the AWS Access keys Secret keys to access s3 bucket,
//    // create a secret with your credentials with keys and the directives
//    // accessKey and secretKey. It can be in any namespace as long as you provide
//    // that namespace in credentialSecretNamespace.
//    CredentialSecretName string `json:"credentialSecretName"`
//    // The namespace of the secret containing the credentials to access S3.
//    // If not set, default to the namespace operator deployed.
//    CredentialSecretNamespace string `json:"credentialSecretNamespace"`
//    // The name of the S3 bucket where backup files will be stored.
//    BucketName string `json:"bucketName"`
//    // The AWS region where the S3 bucket is located.
//    Region string `json:"region"`
//    // The name of the folder in the S3 bucket where backup files will be stored.
//    // Nested folders (e.g., backups/cluster1) are not supported.
//    Folder string `json:"folder"`
//}

//type Minio struct {
//}
