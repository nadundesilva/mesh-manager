# Development Guide for the Mesh Manager

The mesh manager can be deployed into a local cluster with the following steps.

## Setting up

1. Install Operator SDK
   ```bash
   operator-sdk olm install
   ```

## Deploying a build from source

1. Build the mesh manager.
   ```bash
   IMG=nadunrds/mesh-manager-test:dev make docker-build ENABLE_WEBHOOKS=false
   ```
2. Run the following command to install all CRDs
   ```bash
   make install
   ```
3. Deploy the mesh manager.
   ```bash
   IMG=nadunrds/mesh-manager-test:dev make deploy ENABLE_WEBHOOKS=false
   ```
