import { DEFAULT_HTTPS_IMAGE, type HTTPSStatus } from '../../api/publicHTTPS';

export function httpsFixture():HTTPSStatus {
 const id='https-'+'a'.repeat(32);
 return {id,projectId:'p-one',revision:1,reviewKey:'review_'+'b'.repeat(32),lastActionRequestKey:'',actions:[],state:'reviewed',resources:([
  'PersistentVolumeClaim','Secret','ConfigMap','Deployment','Service'] as const).map(kind=>({kind,name:'https-owned',attempted:false,state:'planned' as const,deleteAttempted:false,absentObserved:false})),
  plan:{id,projectId:'p-one',deploymentId:'workload-'+'c'.repeat(32),destination:{targetId:'cluster-'+'d'.repeat(32),targetFingerprint:'e'.repeat(64),namespace:'app',namespaceUid:'namespace-original',serviceId:'app',serviceName:'app-owned',serviceUid:'service-original',servicePort:8080},hostname:'app.example.com',path:'/',pathType:'Prefix',azureDnsZoneId:'/subscriptions/one/resourcegroups/dns/providers/microsoft.network/dnszones/example.com',azureDnsZoneEtag:'etag-original',azureDnsRecordName:'app',azureJobId:'cloud-original',azureJobPlanDigest:'f'.repeat(64),azureClusterUid:'cluster-original',traefikImage:DEFAULT_HTTPS_IMAGE,profile:'traefik-native-acme-azuredns-file-v1',storageClass:'managed-csi',storageClassUid:'storage-original',storageClassVersion:'7',storageBindingMode:'WaitForFirstConsumer',storageGiB:1,acmeEmail:'owner@example.com',dnsClientId:'dns-client',monthlyBudgetUsd:25,createdAt:'2030-01-01T00:00:00Z',expiresAt:'2030-01-01T00:10:00Z',digest:'1'.repeat(64)},
  address:'',dnsAttempted:false,dnsAcknowledged:false,dnsObserved:false,httpsVerified:false,httpStatus:0,applicationHealthVerified:false,observedAt:'',lastAction:'',message:'No resource effects have been attempted.',pvcDataRetained:false};
}
