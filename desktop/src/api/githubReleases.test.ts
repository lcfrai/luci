import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { githubReleasesApi, validGitHubPublication, type GitHubPublication } from './githubReleases';

const fixture=():GitHubPublication=>({id:'gh-release-original',projectId:'p-release',revision:1,intent:{projectId:'p-release',repository:'owner/repo',repositoryId:7,tag:'v1.0.0',commit:'b'.repeat(40),gitReviewDigest:'a'.repeat(64),contextDigest:'a'.repeat(64),artifactId:'artifact-launch',artifactRevision:1,artifactDigest:'a'.repeat(64),artifactSourceDigest:'a'.repeat(64),title:'Reviewed launch',body:'Full public copy\n',account:{id:11,login:'publisher'}},intentDigest:'c'.repeat(64),state:'reviewed',createdAt:'2026-10-31T00:00:00Z',updatedAt:'2026-10-31T00:00:00Z',expiresAt:'2026-10-31T00:10:00Z',draftAttempted:false,publishAttempted:false,remote:null,message:'Reviewed locally.',cliAuthScope:'workspace-cli-not-personal-identity'});
afterEach(()=>vi.restoreAllMocks());

it('rejects foreign project and coerced state and requires exact returned public copy',async()=>{
 const row=fixture();expect(validGitHubPublication(row,'p-release')).toBe(true);
 expect(validGitHubPublication(row,'other')).toBe(false);expect(validGitHubPublication({...row,state:['reviewed']},'p-release')).toBe(false);
 const request=vi.spyOn(productApi,'request').mockResolvedValue({...row,intent:{...row.intent,tag:'v2'}});
 await expect(githubReleasesApi.prepare('p-release',{artifactId:row.intent.artifactId,artifactRevision:1,artifactDigest:row.intent.artifactDigest,tag:'v1.0.0'})).rejects.toThrow('could not be verified');
 request.mockResolvedValue({...row,revision:2,intent:{...row.intent,body:'Different public text'}});
 await expect(githubReleasesApi.act('p-release',row,'draft',true)).rejects.toThrow('could not be verified');
 expect(request.mock.calls[1].slice(0,4)).toEqual(['/v1/product/projects/p-release/distribute/github-releases/gh-release-original/draft',{expectedRevision:1,intentDigest:row.intentDigest,acknowledgeContent:true},'POST',95000]);
});

it('retains exact original remote IDs and separates unknown outcomes from publication',async()=>{
 const row=fixture();row.revision=2;row.draftAttempted=true;row.state='draft-unknown';
 const request=vi.spyOn(productApi,'request').mockResolvedValue({...row,revision:3});
 expect((await githubReleasesApi.act('p-release',row,'observe',false)).state).toBe('draft-unknown');
 const remote={id:42,tag_name:'v1.0.0',target_commitish:row.intent.commit,name:row.intent.title,body:row.intent.body,draft:true,prerelease:false,html_url:'https://github.com/owner/repo/releases/tag/v1.0.0',created_at:row.createdAt,published_at:null,author:row.intent.account};
 row.remote=remote;row.state='draft';request.mockResolvedValue({...row,revision:3,remote:{...remote,id:43}});
 await expect(githubReleasesApi.act('p-release',row,'observe',false)).rejects.toThrow('could not be verified');
 expect(validGitHubPublication({...row,state:'published'},'p-release')).toBe(false);
});
