# Implements hack/version.sh's version::ldflags() for Bazel.
def version_x_defs():
    # This should match the list of packages in version::ldflag
    stamp_pkgs = [
        "kubevirt.io/machine-remediation/pkg/version",
    ]

    # This should match the list of vars in kube::version::ldflags
    # It should also match the list of vars set in hack/print-workspace-status.sh.
    stamp_vars = [
        "buildDate",
        "gitCommit",
        "gitTreeState",
        "gitVersion",
    ]

    # Generate the cross-product.
    x_defs = {}
    for pkg in stamp_pkgs:
        for var in stamp_vars:
            x_defs["%s.%s" % (pkg, var)] = "{%s}" % var
    return x_defs
