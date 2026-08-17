#!/bin/bash
# Quick deployment script for Casbin Gateway on Kubernetes
# This script helps you deploy Casbin Gateway with proper configuration
# Usage: ./deploy.sh [--auto-ingress|--no-ingress]

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse command line arguments
AUTO_INGRESS=""
while [[ $# -gt 0 ]]; do
  case $1 in
    --auto-ingress)
      AUTO_INGRESS="yes"
      shift
      ;;
    --no-ingress)
      AUTO_INGRESS="no"
      shift
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      echo "Usage: $0 [--auto-ingress|--no-ingress]"
      exit 1
      ;;
  esac
done

echo -e "${GREEN}Casbin Gateway Kubernetes Deployment Script${NC}"
echo "==========================================="
echo

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl is not installed${NC}"
    exit 1
fi

# Check if we're in the k8s directory
if [ ! -f "secret.yaml" ] || [ ! -f "deployment.yaml" ]; then
    echo -e "${RED}Error: Please run this script from the k8s directory${NC}"
    exit 1
fi

echo -e "${YELLOW}Step 1: Checking configuration...${NC}"

# Check if secrets have been configured
if grep -q "CHANGEME_INSECURE_DEFAULT_REPLACE_THIS" secret.yaml; then
    echo -e "${RED}Error: Secrets have not been configured!${NC}"
    echo "Please edit k8s/secret.yaml and replace all placeholder values with your actual credentials:"
    echo "  - casdoor-client-id"
    echo "  - casdoor-client-secret"
    exit 1
fi

echo -e "${GREEN}✓ Configuration looks good${NC}"
echo

echo -e "${YELLOW}Step 2: Deploying Casbin Gateway configuration and storage...${NC}"
kubectl apply -f sqlite-pvc.yaml
kubectl apply -f secret.yaml
kubectl apply -f configmap.yaml
echo -e "${GREEN}✓ Configuration deployed${NC}"
echo

echo -e "${YELLOW}Step 3: Deploying Casbin Gateway application...${NC}"
kubectl apply -f deployment.yaml

echo "Waiting for Casbin Gateway to be ready..."
kubectl wait --for=condition=ready pod -l app=caswaf -n caswaf --timeout=300s || {
    echo -e "${RED}Warning: Casbin Gateway took longer than expected to start${NC}"
    echo "Check logs with: kubectl logs -n caswaf -l app=caswaf"
}
echo -e "${GREEN}✓ Casbin Gateway is deployed${NC}"
echo

# Handle Ingress deployment
if [ -z "$AUTO_INGRESS" ]; then
    # Interactive mode - only ask if terminal is interactive
    if [ -t 0 ]; then
        read -p "Do you want to deploy the Ingress? (y/N): " deploy_ingress
        if [[ $deploy_ingress =~ ^[Yy]$ ]]; then
            AUTO_INGRESS="yes"
        else
            AUTO_INGRESS="no"
        fi
    else
        echo -e "${YELLOW}Non-interactive mode detected, skipping Ingress deployment${NC}"
        echo "Use --auto-ingress flag to deploy Ingress automatically"
        AUTO_INGRESS="no"
    fi
fi

if [ "$AUTO_INGRESS" = "yes" ]; then
    echo -e "${YELLOW}Step 4: Deploying Ingress...${NC}"
    kubectl apply -f ingress.yaml
    echo -e "${GREEN}✓ Ingress deployed${NC}"
    echo
    echo -e "${YELLOW}Note: Update your DNS or /etc/hosts to point to your ingress controller IP${NC}"
else
    echo -e "${YELLOW}Skipping Ingress deployment${NC}"
    echo
    echo "You can access Casbin Gateway using port-forward:"
    echo "  kubectl port-forward svc/caswaf 17000:17000 -n caswaf"
    echo "  Then access: http://localhost:17000"
fi

echo
echo -e "${GREEN}Deployment Summary:${NC}"
echo "==================="
kubectl get all -n caswaf

echo
echo -e "${GREEN}✓ Casbin Gateway deployment completed successfully!${NC}"
echo
echo "Useful commands:"
echo "  View pods:        kubectl get pods -n caswaf"
echo "  View services:    kubectl get svc -n caswaf"
echo "  View logs:        kubectl logs -f deployment/caswaf -n caswaf"
echo "  Port forward:     kubectl port-forward svc/caswaf 17000:17000 -n caswaf"
echo "  Delete all:       kubectl delete namespace caswaf"
