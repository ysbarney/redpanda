// Copyright 2021 Vectorized, Inc.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.md
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0

package v1alpha1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	kb = 1024
	mb = 1024 * kb
	gb = 1024 * mb
)

// log is for logging in this package.
var log = logf.Log.WithName("cluster-resource")

// SetupWebhookWithManager autogenerated function by kubebuilder
func (r *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-redpanda-vectorized-io-v1alpha1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=redpanda.vectorized.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=mcluster.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &Cluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
// TODO(user): fill in your defaulting logic.
func (r *Cluster) Default() {
	log.Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-redpanda-vectorized-io-v1alpha1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=redpanda.vectorized.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=vcluster.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Cluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateCreate() error {
	log.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList

	allErrs = append(allErrs, r.validateKafkaPorts()...)

	allErrs = append(allErrs, r.checkCollidingPorts()...)

	allErrs = append(allErrs, r.validateMemory()...)

	allErrs = append(allErrs, r.validateTLS()...)

	allErrs = append(allErrs, r.validateArchivalStorage()...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		r.GroupVersionKind().GroupKind(),
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateUpdate(old runtime.Object) error {
	log.Info("validate update", "name", r.Name)
	oldCluster := old.(*Cluster)
	var allErrs field.ErrorList

	if r.Spec.Replicas != nil && oldCluster.Spec.Replicas != nil && *r.Spec.Replicas < *oldCluster.Spec.Replicas {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec").Child("replicas"),
				r.Spec.Replicas,
				"scaling down is not supported"))
	}

	allErrs = append(allErrs, r.validateKafkaPorts()...)

	allErrs = append(allErrs, r.checkCollidingPorts()...)

	allErrs = append(allErrs, r.validateMemory()...)

	allErrs = append(allErrs, r.validateTLS()...)

	allErrs = append(allErrs, r.validateArchivalStorage()...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		r.GroupVersionKind().GroupKind(),
		r.Name, allErrs)
}

// ReserveMemoryString is amount of memory that we reserve for other processes than redpanda in the container
const ReserveMemoryString = "1M"

func (r *Cluster) validateKafkaPorts() field.ErrorList {
	var allErrs field.ErrorList
	if len(r.Spec.Configuration.KafkaAPI) == 0 {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec").Child("configuration").Child("kafkaApi"),
				r.Spec.Configuration.KafkaAPI,
				"need at least one kafka api listener"))
	}

	var external *KafkaAPIListener
	for i, p := range r.Spec.Configuration.KafkaAPI {
		if p.External.Enabled {
			if external != nil {
				allErrs = append(allErrs,
					field.Invalid(field.NewPath("spec").Child("configuration").Child("kafkaApi"),
						r.Spec.Configuration.KafkaAPI,
						"only one kafka api listener can be marked as external"))
			}
			external = &r.Spec.Configuration.KafkaAPI[i]
			if external.Port != 0 {
				allErrs = append(allErrs,
					field.Invalid(field.NewPath("spec").Child("configuration").Child("kafkaApi"),
						r.Spec.Configuration.KafkaAPI,
						"external kafka api listener cannot have port specified, it's autogenerated"))
			}
		}
	}

	if !((len(r.Spec.Configuration.KafkaAPI) == 2 && external != nil) || (external == nil && len(r.Spec.Configuration.KafkaAPI) == 1)) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec").Child("configuration").Child("kafkaApi"),
				r.Spec.Configuration.KafkaAPI,
				"one internal listener and up to to one external kafka api listener is required"))
	}

	return allErrs
}

// validateMemory verifies that memory limits are aligned with the minimal requirement of redpanda
// which is 1GB per core
// to verify this, we need to subtract the 1M we reserve currently for other processes
func (r *Cluster) validateMemory() field.ErrorList {
	var allErrs field.ErrorList
	quantity := resource.MustParse(ReserveMemoryString)
	if !r.Spec.Configuration.DeveloperMode && (r.Spec.Resources.Limits.Memory().Value()-quantity.Value()) < gb {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec").Child("resources").Child("limits").Child("memory"),
				r.Spec.Resources.Limits.Memory(),
				"need minimum of 1GB + 1MB of memory per node"))
	}
	return allErrs
}

func (r *Cluster) validateTLS() field.ErrorList {
	var allErrs field.ErrorList
	if r.Spec.Configuration.TLS.KafkaAPI.RequireClientAuth && !r.Spec.Configuration.TLS.KafkaAPI.Enabled {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec").Child("configuration").Child("tls").Child("requireclientauth"),
				r.Spec.Configuration.TLS.KafkaAPI.RequireClientAuth,
				"Enabled has to be set to true for RequireClientAuth to be allowed to be true"))
	}
	if r.Spec.Configuration.TLS.KafkaAPI.IssuerRef != nil && r.Spec.Configuration.TLS.KafkaAPI.NodeSecretRef != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec").Child("configuration").Child("tls").Child("nodeSecretRef"),
				r.Spec.Configuration.TLS.KafkaAPI.NodeSecretRef,
				"Cannot provide both IssuerRef and NodeSecretRef"))
	}
	return allErrs
}

func (r *Cluster) validateArchivalStorage() field.ErrorList {
	var allErrs field.ErrorList
	if !r.Spec.CloudStorage.Enabled {
		return allErrs
	}
	if r.Spec.CloudStorage.AccessKey == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec").Child("configuration").Child("cloudStorage").Child("accessKey"),
				r.Spec.CloudStorage.AccessKey,
				"AccessKey has to be provided for cloud storage to be enabled"))
	}
	if r.Spec.CloudStorage.Bucket == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec").Child("configuration").Child("cloudStorage").Child("bucket"),
				r.Spec.CloudStorage.Bucket,
				"Bucket has to be provided for cloud storage to be enabled"))
	}
	if r.Spec.CloudStorage.Region == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec").Child("configuration").Child("cloudStorage").Child("region"),
				r.Spec.CloudStorage.Region,
				"Region has to be provided for cloud storage to be enabled"))
	}
	if r.Spec.CloudStorage.SecretKeyRef.Name == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec").Child("configuration").Child("cloudStorage").Child("secretKeyRef").Child("name"),
				r.Spec.CloudStorage.SecretKeyRef.Name,
				"SecretKeyRef name has to be provided for cloud storage to be enabled"))
	}
	if r.Spec.CloudStorage.SecretKeyRef.Namespace == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec").Child("configuration").Child("cloudStorage").Child("secretKeyRef").Child("namespace"),
				r.Spec.CloudStorage.SecretKeyRef.Namespace,
				"SecretKeyRef namespace has to be provided for cloud storage to be enabled"))
	}
	return allErrs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateDelete() error {
	log.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *Cluster) checkCollidingPorts() field.ErrorList {
	var allErrs field.ErrorList

	for _, kafka := range r.Spec.Configuration.KafkaAPI {
		if r.Spec.Configuration.AdminAPI.Port == kafka.Port {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("configuration", "admin", "port"),
					r.Spec.Configuration.AdminAPI.Port,
					"admin port collide with Spec.Configuration.KafkaAPI Port"))
		}
		if r.Spec.ExternalConnectivity.Enabled && r.Spec.Configuration.AdminAPI.Port+1 == kafka.Port {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("configuration", "admin", "port"),
					r.Spec.Configuration.AdminAPI.Port,
					"external admin port collide with Spec.Configuration.KafkaAPI Port"))
		}
	}

	for _, kafka := range r.Spec.Configuration.KafkaAPI {
		if r.Spec.Configuration.RPCServer.Port == kafka.Port {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("configuration", "rpcServer", "port"),
					r.Spec.Configuration.RPCServer.Port,
					"rpc port collide with Spec.Configuration.KafkaAPI Port"))
		}
	}

	if r.Spec.Configuration.AdminAPI.Port == r.Spec.Configuration.RPCServer.Port {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec").Child("configuration", "admin", "port"),
				r.Spec.Configuration.AdminAPI.Port,
				"admin port collide with Spec.Configuration.RPCServer.Port"))
	}

	for _, kafka := range r.Spec.Configuration.KafkaAPI {
		if r.ExternalListener() != nil && kafka.Port+1 == r.Spec.Configuration.RPCServer.Port {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("configuration", "rpcServer", "port"),
					r.Spec.Configuration.RPCServer.Port,
					"rpc port collide with external Kafka API that is not visible in the Cluster CR"))
		}
	}

	for _, kafka := range r.Spec.Configuration.KafkaAPI {
		if r.ExternalListener() != nil && kafka.Port+1 == r.Spec.Configuration.AdminAPI.Port {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("configuration", "admin", "port"),
					r.Spec.Configuration.AdminAPI.Port,
					"admin port collide with external Kafka API that is not visible in the Cluster CR"))
		}
	}

	if r.Spec.ExternalConnectivity.Enabled && r.Spec.Configuration.AdminAPI.Port+1 == r.Spec.Configuration.RPCServer.Port {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec").Child("configuration", "rpcServer", "port"),
				r.Spec.Configuration.RPCServer.Port,
				"rpc port collides with external Admin API port that is not visible in the Cluster CR"))
	}

	for _, kafka := range r.Spec.Configuration.KafkaAPI {
		if r.ExternalListener() != nil && r.Spec.ExternalConnectivity.Enabled && r.Spec.Configuration.AdminAPI.Port+1 == kafka.Port+1 {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("configuration", "kafka", "port"),
					kafka.Port,
					"kafka port collides with external Admin API port that is not visible in the Cluster CR"))
		}
	}

	return allErrs
}
