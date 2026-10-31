import type { BlueprintHealth, BlueprintProbe } from '../../api/deploymentBlueprint';

const freshProbe=():BlueprintProbe=>({kind:'http',path:'',initialDelaySeconds:0,periodSeconds:10,timeoutSeconds:1,failureThreshold:3});

export function InferenceHealthEditor({value,port,onChange}:{value?:BlueprintHealth;port:number;onChange(value:BlueprintHealth|undefined):void}) {
  function change(kind:'startup'|'readiness',probe:BlueprintProbe|undefined) {
    const next={...value,[kind]:probe};onChange(next.startup||next.readiness?next:undefined);
  }
  return <details><summary>Startup and readiness checks</summary>
    <p className="journey-muted">Use checks that your image actually serves on container port {port||'not selected'}. No exec command, alternate host, request headers or credentials are supported. A ready probe does not verify model contents or response quality.</p>
    {(['startup','readiness'] as const).map(kind=>{
      const probe=value?.[kind],label=kind==='startup'?'Startup':'Readiness';
      return <fieldset key={kind}><legend>{label} probe</legend>
        <label><input type="checkbox" checked={!!probe} onChange={event=>change(kind,event.target.checked?freshProbe():undefined)}/>Configure {kind} probe</label>
        {probe&&<>
          <label className="luci-field">{label} protocol<select value={probe.kind} onChange={event=>change(kind,{...probe,kind:event.target.value==='tcp'?'tcp':'http',path:event.target.value==='tcp'?undefined:''})}><option value="http">HTTP path on declared port</option><option value="tcp">TCP on declared port</option></select></label>
          {probe.kind==='http'&&<label className="luci-field">{label} HTTP path<input value={probe.path??''} placeholder="Your image’s health path" maxLength={512} onChange={event=>change(kind,{...probe,path:event.target.value})}/></label>}
          <div className="blueprint-grid">{(['initialDelaySeconds','periodSeconds','timeoutSeconds','failureThreshold'] as const).map((field,index)=><label className="luci-field" key={field}>{label} {['initial delay (seconds)','period (seconds)','timeout (seconds)','failure threshold'][index]}<input type="number" min={index===0?0:1} max={[600,300,30,120][index]} step={1} value={probe[field]} onChange={event=>change(kind,{...probe,[field]:event.target.value===''?0:Number(event.target.value)})}/></label>)}</div>
        </>}
      </fieldset>;
    })}
  </details>;
}
