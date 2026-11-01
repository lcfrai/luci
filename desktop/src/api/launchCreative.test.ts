import { describe,expect,it } from 'vitest';
import { creativeArtifactValid,creativeEditorDraftValid,creativeSVGBlob,type LaunchCreativeBrief } from './launchCreative';

const brief:LaunchCreativeBrief={product:'Product',headline:'A useful product',body:'For your launch',cta:'Learn more',alt:'Launch visual',format:'wide',background:'#ffffff',foreground:'#18181b',accent:'#244f44',frameId:'',assetId:''};
describe('launch creative boundaries',()=>{
  it('accepts only the closed self-contained image grammar',()=>{
    const svg='<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="675"><title>Product</title><rect width="1200" height="675" fill="#ffffff"/><text x="20" y="40">Launch</text></svg>';
    expect(creativeSVGBlob(svg).type).toBe('image/svg+xml');
    expect(()=>creativeSVGBlob(svg.replace('<text','<script').replace('</text>','</script>'))).toThrow();
    expect(()=>creativeSVGBlob(svg.replace('</svg>','<image href="https://example.com/private"/></svg>'))).toThrow();
    expect(()=>creativeSVGBlob(svg.replace('</svg>','<foreignObject/></svg>'))).toThrow();
    expect(()=>creativeSVGBlob(svg.replace('fill="#ffffff"','fill="url(https://example.com/paint)"'))).toThrow();
    expect(()=>creativeSVGBlob('<?xml-stylesheet href="https://example.com/style"?>'+svg)).toThrow();
  });
  it('retains a complete original pending brief while refusing a substituted payload or project',()=>{
    const record={projectId:'p-one',brief,saved:null,pending:{mode:'create',key:'creative-11111111-1111-4111-8111-111111111111',brief,prepareDigest:'a'.repeat(64)}};
    expect(creativeEditorDraftValid(record,'p-one')).toBe(true);
    expect(creativeEditorDraftValid(record,'p-other')).toBe(false);
    expect(creativeEditorDraftValid({...record,pending:{...record.pending,brief:{...brief,headline:'Different request'}}},'p-one')).toBe(false);
    expect(creativeEditorDraftValid({...record,pending:{...record.pending,externalURL:'https://example.com'}},'p-one')).toBe(false);
  });
});

it('refuses malformed restored frame geometry and labels before rendering UI',()=>{
 const frame={id:'frame-one',name:'Product',screenId:'screen-one',state:'',width:640,height:360,background:'#ffffff',nodes:[]};
 const row={id:'artifact-one',projectId:'p-one',revision:1,kind:'launch-creative',title:'Launch',state:'draft',digest:'a'.repeat(64),sourceDigest:'b'.repeat(64),source:{designRevision:1,designDigest:'c'.repeat(64),gtmRevision:1,gtmDigest:'d'.repeat(64)},sections:[],gaps:[],assumptions:[],creative:{version:1,renderer:'luci-launch-svg-v1',brief:{...brief,frameId:frame.id},frame,frameDigest:'e'.repeat(64),assets:[]}};
 expect(creativeArtifactValid(row,'p-one')).toBe(true);
 expect(creativeArtifactValid({...row,creative:{...row.creative,frame:{...frame,width:NaN}}},'p-one')).toBe(false);
 expect(creativeArtifactValid({...row,creative:{...row.creative,frame:{...frame,name:{unexpected:'object'}}}},'p-one')).toBe(false);
 expect(creativeArtifactValid({...row,title:{unexpected:'object'}},'p-one')).toBe(false);
});
