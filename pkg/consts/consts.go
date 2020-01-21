package consts

const (
	// AnnotationBareMetalHost contains the annotation key for bare metal host
	AnnotationBareMetalHost = "metal3.io/BareMetalHost"
	// AnnotationMachine contains the annotation key for machine
	AnnotationMachine = "machine.openshift.io/machine"
	// AnnotationNodeMachineReboot contains machine reboot annotation key, once nodereboot controller will detect it,
	// it will create the MachineRemediation object
	AnnotationNodeMachineReboot = "healthchecking.openshift.io/machine-remediation-reboot"
	// AnnotationRebootInProgress contains the annotation key, that indicates that reboot in the progress
	AnnotationRebootInProgress = "machineremediation.kubevirt.io/rebootInProgress"
	//MachineRoleLabel contains machine role label
	MachineRoleLabel = "machine.openshift.io/cluster-api-machine-role"
	// MasterMachineHealthCheck contains the MachineHealthCheck name for master nodes
	MasterMachineHealthCheck = "masters"
	// MasterMachineDisruptionBudget contains the MachineDisruptionBudget name for master nodes
	MasterMachineDisruptionBudget = "masters"
	// NamespaceOpenshiftMachineAPI contains namespace name for the machine-api componenets under the OpenShift cluster
	NamespaceOpenshiftMachineAPI = "openshift-machine-api"
	//NodeMasterRoleLabel contains node master role label
	NodeMasterRoleLabel = "node-role.kubernetes.io/master"
)
