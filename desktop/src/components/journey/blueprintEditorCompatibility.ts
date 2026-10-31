import type { DeploymentBlueprint } from '../../api/deploymentBlueprint';

const only=(value:object,keys:string[])=>Object.keys(value).every(key=>keys.includes(key));
// A readable newer saved record is not permission to discard fields this editor
// cannot represent. Keep the original saved object available for export and
// block all replacements, including baseline-review and Clear draft paths.
export function blueprintEditorRecognizesFields(b:DeploymentBlueprint):boolean {
 const probe=(value:object)=>only(value,['kind','path','initialDelaySeconds','periodSeconds','timeoutSeconds','failureThreshold']);
 const tls=(value:object)=>only(value,['resourceName','caKey','certKey','keyKey']);
 return only(b,['version','projectId','source','infrastructure','services','externalResources','publicRoute']) &&
  (!b.publicRoute||b.publicRoute.profile==='traefik-websecure'&&only(b.publicRoute,['profile','serviceId','ingressClassName','hostname','path','pathType','tlsSecretName'])) &&
  only(b.source,['commit','configurationRevision','context']) && (!b.source.context||only(b.source.context,['briefRevision','specificationRevision','specificationDigest','designRevision','designDigest'])) &&
  only(b.infrastructure,['id','kind','reference','namespace']) && (b.externalResources??[]).every(row=>only(row,['id','kind','reference'])) &&
  b.services.every(row=>only(row,['id','kind','image','port','replicas','resources','command','args','dependsOn','model','telemetry','collector','telemetryStorage','environment','health','deploymentStrategy','imagePullSecrets']) &&
   only(row.resources,['requestMilliCpu','limitMilliCpu','requestMemoryMiB','limitMemoryMiB','gpuCount']) &&
   (!row.model||only(row.model,['name','claimName','mountPath','relativePath','accessGroup'])) && (!row.telemetry||only(row.telemetry,['resourceId','endpoint','clientTLS','instrumentation'])&&(!row.telemetry.clientTLS||tls(row.telemetry.clientTLS))&&(!row.telemetry.instrumentation||only(row.telemetry.instrumentation,['runtime','dependencies']))) &&
   (!row.telemetryStorage||only(row.telemetryStorage,['profile','claimName','capacityGiB','retentionHours','serverTLS','queryTLS','azure'])&&tls(row.telemetryStorage.serverTLS)&&tls(row.telemetryStorage.queryTLS)&&(!row.telemetryStorage.azure||only(row.telemetryStorage.azure,['accountName','containerName','accountKey'])&&only(row.telemetryStorage.azure.accountKey,['resourceName','key']))) &&
   (!row.collector||only(row.collector,['profile','destination','ownedDestination','receiverTLS','events'])&&(!row.collector.ownedDestination||only(row.collector.ownedDestination,['metricsServiceId','tracesServiceId','clientTLS'])&&tls(row.collector.ownedDestination.clientTLS))&&only(row.collector.destination,['resourceId','endpoint','authorization'])&&(!row.collector.destination.authorization||only(row.collector.destination.authorization,['resourceName','key']))&&tls(row.collector.receiverTLS)&&row.collector.events.every(event=>only(event,['id','description']))) &&
   (row.environment??[]).every(ref=>only(ref,['name','kind','resourceName','key'])) &&
   (!row.health||only(row.health,['startup','readiness'])&&(!row.health.startup||probe(row.health.startup))&&(!row.health.readiness||probe(row.health.readiness))));
}
