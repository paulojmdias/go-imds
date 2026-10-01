// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"context"
	"fmt"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

// apiVersion is pinned rather than tracking the newest. The service rejects an
// unknown version outright, and a version pinned here is one that every field
// below is known to exist in.
// The whole instance document rather than just its compute half, so that the
// network interfaces come back in the same request.
const metadataPath = "/metadata/instance?api-version=2021-12-13&format=json"

// Metadata describes the Azure virtual machine the process is running on.
type Metadata struct {
	// ID is the VM's unique identifier (vmId).
	ID string
	// Hostname is the operating system computer name, falling back to the VM
	// name when the image was specialised and reports none.
	Hostname string
	// Name is the VM resource name.
	Name string
	// Region is the Azure location, for example "westeurope".
	Region string
	// Zone is the availability zone, empty for a VM not placed in one.
	Zone string
	// ResourceID is the fully qualified ARM resource identifier.
	ResourceID string
	// AccountID is the subscription the VM belongs to.
	AccountID string
	// Type is the VM size, for example "Standard_D2s_v3".
	Type string
	// OSType and OSVersion describe the operating system image.
	OSType    string
	OSVersion string
	// ResourceGroupName is the resource group containing the VM.
	ResourceGroupName string
	// ScaleSetName is the scale set the VM belongs to, empty when it is not in
	// one.
	ScaleSetName string
	// Tags are the VM's tags, keyed by the raw Azure tag name.
	Tags map[string]string

	// Offer, Publisher, SKU and OSVersion identify the marketplace image. They
	// are empty for a VM created from a custom image.
	Offer     string
	Publisher string
	SKU       string
	// AZEnvironment is the Azure cloud the VM runs in, for example
	// "AzurePublicCloud".
	AZEnvironment string
	// PhysicalZone is the platform's own zone name, which differs from Zone.
	PhysicalZone string
	// PlacementGroupID, PlatformFaultDomain and PlatformUpdateDomain place the
	// VM within the platform's failure and update domains.
	PlacementGroupID     string
	PlatformFaultDomain  string
	PlatformUpdateDomain string
	// Priority is "Regular" or "Spot", and EvictionPolicy applies to Spot only.
	Priority       string
	EvictionPolicy string
	// LicenseType is set for a VM using Azure Hybrid Benefit.
	LicenseType string
	// Provider is the resource provider, normally "Microsoft.Compute".
	Provider string
	// AdminUsername is the administrator account name. The password is in the
	// document and is deliberately not read; see the package documentation.
	AdminUsername string
	// DisablePasswordAuthentication reports whether password login is off.
	DisablePasswordAuthentication string
	// VirtualMachineScaleSetID is the scale set's resource ID.
	VirtualMachineScaleSetID string
	// PublicKeys are the SSH public keys installed on the VM.
	PublicKeys []PublicKey
	// SecurityProfile describes trusted-launch and confidential-compute
	// settings.
	SecurityProfile SecurityProfile
	// StorageProfile describes the OS disk, data disks and image reference.
	StorageProfile StorageProfile
	// Plan is the marketplace purchase plan, empty for most VMs.
	Plan Plan
	// Interfaces are the VM's network interfaces.
	Interfaces []Interface
}

// PublicKey is one SSH public key installed on the VM.
type PublicKey struct {
	KeyData string `json:"keyData"`
	Path    string `json:"path"`
}

// SecurityProfile describes trusted-launch and confidential-compute settings.
// The service reports these as strings rather than booleans.
type SecurityProfile struct {
	SecureBootEnabled string `json:"secureBootEnabled"`
	VirtualTpmEnabled string `json:"virtualTpmEnabled"`
	EncryptionAtHost  string `json:"encryptionAtHost"`
	SecurityType      string `json:"securityType"`
}

// Plan is a marketplace purchase plan.
type Plan struct {
	Name      string `json:"name"`
	Product   string `json:"product"`
	Publisher string `json:"publisher"`
}

// StorageProfile describes the VM's disks and the image they came from.
type StorageProfile struct {
	ImageReference ImageReference `json:"imageReference"`
	OSDisk         OSDisk         `json:"osDisk"`
	DataDisks      []DataDisk     `json:"dataDisks"`
	ResourceDisk   struct {
		Size string `json:"size"`
	} `json:"resourceDisk"`
}

// ImageReference identifies the image the VM was created from.
type ImageReference struct {
	ID        string `json:"id"`
	Offer     string `json:"offer"`
	Publisher string `json:"publisher"`
	SKU       string `json:"sku"`
	Version   string `json:"version"`
}

// OSDisk describes the operating-system disk. Sizes are in gibibytes, as
// strings, which is how the service reports them.
type OSDisk struct {
	Name                    string `json:"name"`
	Caching                 string `json:"caching"`
	CreateOption            string `json:"createOption"`
	DiskSizeGB              string `json:"diskSizeGB"`
	OSType                  string `json:"osType"`
	WriteAcceleratorEnabled string `json:"writeAcceleratorEnabled"`
	ManagedDisk             struct {
		ID                 string `json:"id"`
		StorageAccountType string `json:"storageAccountType"`
	} `json:"managedDisk"`
}

// DataDisk describes one attached data disk.
type DataDisk struct {
	Name                    string `json:"name"`
	Caching                 string `json:"caching"`
	CreateOption            string `json:"createOption"`
	DiskSizeGB              string `json:"diskSizeGB"`
	Lun                     string `json:"lun"`
	WriteAcceleratorEnabled string `json:"writeAcceleratorEnabled"`
	ManagedDisk             struct {
		ID                 string `json:"id"`
		StorageAccountType string `json:"storageAccountType"`
	} `json:"managedDisk"`
}

// Interface is one network interface.
type Interface struct {
	MACAddress string `json:"macAddress"`
	IPv4       struct {
		IPAddress []IPAddress `json:"ipAddress"`
		Subnet    []Subnet    `json:"subnet"`
	} `json:"ipv4"`
	IPv6 struct {
		IPAddress []IPAddress `json:"ipAddress"`
	} `json:"ipv6"`
}

// IPAddress is one address on an interface. PublicIPAddress is empty for an
// interface with no public address.
type IPAddress struct {
	PrivateIPAddress string `json:"privateIpAddress"`
	PublicIPAddress  string `json:"publicIpAddress"`
}

// Subnet is the subnet an interface is attached to.
type Subnet struct {
	Address string `json:"address"`
	Prefix  string `json:"prefix"`
}

type document struct {
	Compute compute `json:"compute"`
	Network struct {
		Interface []Interface `json:"interface"`
	} `json:"network"`
}

type compute struct {
	VMID              string `json:"vmId"`
	Location          string `json:"location"`
	ResourceID        string `json:"resourceId"`
	Name              string `json:"name"`
	VMSize            string `json:"vmSize"`
	OSType            string `json:"osType"`
	Version           string `json:"version"`
	SubscriptionID    string `json:"subscriptionId"`
	ResourceGroupName string `json:"resourceGroupName"`
	VMScaleSetName    string `json:"vmScaleSetName"`
	Zone              string `json:"zone"`
	OSProfile         struct {
		ComputerName                  string `json:"computerName"`
		AdminUsername                 string `json:"adminUsername"`
		DisablePasswordAuthentication string `json:"disablePasswordAuthentication"`
	} `json:"osProfile"`
	TagsList []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"tagsList"`

	Offer                  string          `json:"offer"`
	Publisher              string          `json:"publisher"`
	SKU                    string          `json:"sku"`
	AZEnvironment          string          `json:"azEnvironment"`
	PhysicalZone           string          `json:"physicalZone"`
	PlacementGroupID       string          `json:"placementGroupId"`
	PlatformFaultDomain    string          `json:"platformFaultDomain"`
	PlatformUpdateDomain   string          `json:"platformUpdateDomain"`
	Priority               string          `json:"priority"`
	EvictionPolicy         string          `json:"evictionPolicy"`
	LicenseType            string          `json:"licenseType"`
	Provider               string          `json:"provider"`
	PublicKeys             []PublicKey     `json:"publicKeys"`
	SecurityProfile        SecurityProfile `json:"securityProfile"`
	StorageProfile         StorageProfile  `json:"storageProfile"`
	Plan                   Plan            `json:"plan"`
	VirtualMachineScaleSet struct {
		ID string `json:"id"`
	} `json:"virtualMachineScaleSet"`
}

// Fetch reads the metadata of the Azure virtual machine the process is running
// on.
//
// The error is [imds.ErrNotDetected] when the process is not on an Azure VM,
// including when the shared address is answered by a metadata service
// belonging to another cloud. Any other error means Azure's service was
// reachable and the lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		if _, err := c.GetJSONFirst(ctx, c.Endpoints(defaultEndpoints...), metadataPath, &doc,
			imds.WithHeader("Metadata", "True")); err != nil {
			return err
		}
		// vmId is present on every Azure VM, so its absence means the document
		// came from something else answering on the shared address.
		if doc.Compute.VMID == "" {
			return fmt.Errorf("%w: no compute.vmId in the metadata document", imds.ErrForeign)
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	// A VM created from a specialised disk reports no computer name.
	cm := doc.Compute
	hostname := cm.OSProfile.ComputerName
	if hostname == "" {
		hostname = cm.Name
	}

	md := Metadata{
		ID:                doc.Compute.VMID,
		Hostname:          hostname,
		Name:              cm.Name,
		Region:            cm.Location,
		Zone:              cm.Zone,
		ResourceID:        cm.ResourceID,
		AccountID:         cm.SubscriptionID,
		Type:              cm.VMSize,
		OSType:            cm.OSType,
		OSVersion:         cm.Version,
		ResourceGroupName: cm.ResourceGroupName,
		ScaleSetName:      cm.VMScaleSetName,

		Offer:                         cm.Offer,
		Publisher:                     cm.Publisher,
		SKU:                           cm.SKU,
		AZEnvironment:                 cm.AZEnvironment,
		PhysicalZone:                  cm.PhysicalZone,
		PlacementGroupID:              cm.PlacementGroupID,
		PlatformFaultDomain:           cm.PlatformFaultDomain,
		PlatformUpdateDomain:          cm.PlatformUpdateDomain,
		Priority:                      cm.Priority,
		EvictionPolicy:                cm.EvictionPolicy,
		LicenseType:                   cm.LicenseType,
		Provider:                      cm.Provider,
		AdminUsername:                 cm.OSProfile.AdminUsername,
		DisablePasswordAuthentication: cm.OSProfile.DisablePasswordAuthentication,
		VirtualMachineScaleSetID:      cm.VirtualMachineScaleSet.ID,
		PublicKeys:                    cm.PublicKeys,
		SecurityProfile:               cm.SecurityProfile,
		StorageProfile:                cm.StorageProfile,
		Plan:                          cm.Plan,
		Interfaces:                    doc.Network.Interface,
	}
	if len(cm.TagsList) > 0 {
		md.Tags = make(map[string]string, len(cm.TagsList))
		for _, t := range cm.TagsList {
			md.Tags[t.Name] = t.Value
		}
	}
	return md, nil
}
