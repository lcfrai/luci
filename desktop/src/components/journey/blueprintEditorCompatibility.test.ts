import { expect, it } from 'vitest';
import { blueprintEditorRecognizesFields } from './blueprintEditorCompatibility';
import { clusterReviewFixture } from '../../test/kubernetesDeploymentFixtures';

it('represents optional inference fields without mutating the saved object',()=>{
 const b=clusterReviewFixture().request.blueprint;
 b.services[0].health={readiness:{kind:'tcp',initialDelaySeconds:0,periodSeconds:5,timeoutSeconds:1,failureThreshold:3}};
 b.services[0].environment=[{name:'TOKEN',kind:'secret',resourceName:'access',key:'token'}];b.services[0].deploymentStrategy='recreate';
 const before=JSON.stringify(b);expect(blueprintEditorRecognizesFields(b)).toBe(true);expect(JSON.stringify(b)).toBe(before);
});
it('blocks narrower editor replacement when saved root or nested fields are unrecognized',()=>{
 const b=clusterReviewFixture().request.blueprint;
 const root={...b,futureTopology:{keep:true}};expect(blueprintEditorRecognizesFields(root)).toBe(false);
 const infrastructure={...b,infrastructure:{...b.infrastructure,futurePlacement:'keep'}};expect(blueprintEditorRecognizesFields(infrastructure)).toBe(false);
 const nested={...b,services:[{...b.services[0],resources:{...b.services[0].resources,futureAccelerator:1}}]};expect(blueprintEditorRecognizesFields(nested)).toBe(false);
});
