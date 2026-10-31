import {expect,it} from 'vitest';
import {validBlueprintPublicRoute,type BlueprintPublicRoute} from './deploymentBlueprint';

const route=():BlueprintPublicRoute=>({profile:'traefik-websecure',serviceId:'web',ingressClassName:'traefik',hostname:'app.example.com',path:'/',pathType:'Prefix',tlsSecretName:'site-tls'});
it('accepts a closed existing-controller route with exact backend selection and names only',()=>{
 expect(validBlueprintPublicRoute(route())).toBe(true);
 expect(validBlueprintPublicRoute({...route(),path:'/app',pathType:'Exact',ingressClassName:'123-traefik',tlsSecretName:'site.tls'})).toBe(true);
});
it.each([
 {hostname:'127.0.0.1'}, {hostname:'999.123.456.789'}, {hostname:'*.example.com'},
 {hostname:'app.internal'}, {path:'/?key=secret'}, {path:'/a%2fb'}, {path:'/a/../b'},
 {profile:['traefik-websecure']}, {pathType:['Prefix']},
])('rejects unsupported route or credential channels %#',change=>{
 expect(validBlueprintPublicRoute({...route(),...change})).toBe(false);
});
