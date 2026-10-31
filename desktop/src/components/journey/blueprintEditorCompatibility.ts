import type { DeploymentBlueprint } from '../../api/deploymentBlueprint';

const only=(value:object,keys:string[])=>Object.keys(value).every(key=>keys.includes(key));
// A readable newer saved record is not permission to discard fields this editor
// cannot represent. Keep the original saved object available for export and
// block all replacements, including baseline-review and Clear draft paths.
export function blueprintEditorRecognizesFields(b:DeploymentBlueprint):boolean {
 const probe=(value:object)=>only(value,['kind','path','initialDelaySeconds','periodSeconds','timeoutSeconds','failureThreshold']);
 const tls=(value:object)=>only(value,['resourceName','caKey','certKey','keyKey']);
 return only(b,['version','projectId','source','infrastructure','services','externalResources']) &&
  only(b.source,['commit','configurationRevision','context']) && (!b.source.context||only(b.source.context,['briefRevision','specificationRevision','specificationDigest','designRevision','designDigest'])) &&
  only(b.infrastructure,['id','kind','reference','namespace']) && (b.externalResources??[]).every(row=>only(row,['id','kind','reference'])) &&
  b.services.every(row=>only(row,['id','kind','image','port','replicas','resources','command','args','dependsOn','model','telemetry','collector','environment','health','deploymentStrategy']) &&
   only(row.resources,['requestMilliCpu','limitMilliCpu','requestMemoryMiB','limitMemoryMiB','gpuCount']) &&
   (!row.model||only(row.model,['name','claimName','mountPath','relativePath'])) && (!row.telemetry||only(row.telemetry,['resourceId','endpoint','clientTLS','instrumentation'])&&(!row.telemetry.clientTLS||tls(row.telemetry.clientTLS))&&(!row.telemetry.instrumentation||only(row.telemetry.instrumentation,['runtime','dependencies']))) &&
   (!row.collector||only(row.collector,['profile','destination','receiverTLS','events'])&&only(row.collector.destination,['resourceId','endpoint','authorization'])&&(!row.collector.destination.authorization||only(row.collector.destination.authorization,['resourceName','key']))&&tls(row.collector.receiverTLS)&&row.collector.events.every(event=>only(event,['id','description']))) &&
   (row.environment??[]).every(ref=>only(ref,['name','kind','resourceName','key'])) &&
   (!row.health||only(row.health,['startup','readiness'])&&(!row.health.startup||probe(row.health.startup))&&(!row.health.readiness||probe(row.health.readiness))));
}
