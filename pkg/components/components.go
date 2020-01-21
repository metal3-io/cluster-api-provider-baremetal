package components

const (
	// ComponentMachineRemediation contains name for MachineRemediation component
	ComponentMachineRemediation = "machine-remediation"
)

var (
	// Components contains names of all componenets that the operator should deploy
	Components = []string{
		ComponentMachineRemediation,
	}
)

const (
	// CRDMachineRemediation contains the kind of the MachineRemediation CRD
	CRDMachineRemediation = "machineremediations"
)

var (
	// CRDS contains names of all CRD's that the operator should deploy
	CRDS = []string{
		CRDMachineRemediation,
	}
)
