# Fix unlock-timestamp Annotation Cleanup Issue

## Problem Description

When manually setting the `clawcloud.run/status` label of a namespace to `active`, the `clawcloud.run/unlock-timestamp` annotation is not automatically cleaned up, causing the annotation to remain.

## Root Cause

The optimized architecture is primarily driven by BlockRequest events. Direct label changes may not trigger the controller's reconciliation loop to clean up annotations.

## Solutions

### Solution 1: Upgrade to v0.1.5 (Recommended)

Version v0.1.5 enhances the scanner's log output and annotation cleanup logic:

```bash
# Upgrade to the fixed version
kubectl apply -f deploy/block/deployment-simple.yaml
```

After upgrading, the scanner will:
1. Detect namespaces with `active` status
2. Output detailed logs of the annotation cleanup process
3. Automatically clean up the `unlock-timestamp` annotation

### Solution 2: Manual Cleanup (Immediate Resolution)

Use the provided cleanup script:

```bash
# Run the cleanup script
./scripts/cleanup-annotations.sh
```

Or clean up manually:

```bash
# Find problematic namespaces
kubectl get namespaces -o custom-columns=NAME:.metadata.name | xargs -I {} sh -c 'kubectl get namespace {} -o jsonpath="{.metadata.annotations.clawcloud\.run/unlock-timestamp}" && echo " {}"'

# Manually clean up a specific namespace
kubectl annotate namespace your-namespace clawcloud.run/unlock-timestamp-
```

### Solution 3: Use kubectl One-line Command

```bash
# Clean up all namespaces that have active status but still have unlock-timestamp annotation
kubectl get namespaces -o json | \
  jq -r '.items[] | select(.metadata.labels."clawcloud.run/status" == "active" and .metadata.annotations."clawcloud.run/unlock-timestamp") | .metadata.name' | \
  xargs -I {} kubectl annotate namespace {} clawcloud.run/unlock-timestamp-
```

## Verification

After upgrading, check the logs to confirm annotations were cleaned up:

```bash
kubectl logs -n block-system deployment/block-controller | grep "unlock-timestamp"
```

You should see logs similar to:
```
"namespace is active, handling unlock" {"hasUnlockTimestamp": true}
"removing unlock-timestamp annotation"
"successfully removed unlock-timestamp annotation"
```

## Preventive Measures

1. **Use BlockRequest CRD**: It is recommended to use BlockRequest for blocking/unblocking instead of directly modifying labels
2. **Regular Checks**: You can run the cleanup script periodically as a preventive measure
3. **Monitor Logs**: Pay attention to the scanner's log output

## Version Information

- **Fixed Version**: v0.1.5
- **Affected Versions**: v0.1.2 - v0.1.4
- **Fix Details**: Enhanced annotation cleanup logic and log output
