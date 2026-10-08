async function seedProjects(connectionID){let project,projectRoute,run;
  for(let index=0;index<2;index++){
   const letter='abcd'[index],task='build-'+letter;titles.set(task,'Build project '+letter.toUpperCase()+' result');
  project=await api('POST',workspace+'/v1/product/projects',{name:'recovery-project-'+letter},201);assert(realpathSync(project.path).startsWith(fixture+sep));projectRoute=workspace+'/v1/product/projects/'+project.id;
  await api('POST',projectRoute+'/brief',{revision:0,purpose:'Prove '+task+' uses its own project context and reviewed integration under a shared worker budget',audience:'Project owners',constraints:'Local labelled fixture; no cloud or model calls',criteria:['Only this project assigned file is changed and reviewed before integration']});
  let spec=await api('GET',projectRoute+'/specification');spec=await api('POST',projectRoute+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,requirements:[{id:'AC-notes',title:'Coordinate isolated task work with reviewed integration',priority:'must',acceptance:'Each task changes only its assigned file using its own captured project context; canceled work never allocates a worker.'}]}});
  spec=await api('POST',projectRoute+'/specification/approve',{revision:spec.revision,briefRevision:spec.briefRevision});
  let design=await api('GET',projectRoute+'/design-plan');design=await api('POST',projectRoute+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:spec.briefRevision,expectedSpecificationRevision:spec.revision,plan:{...design,screens:[{id:'notes',name:'Saved notes',route:'/',purpose:'Save and restore a note',states:['Empty','Saved','Error and retry'],widths:[390,1440],requirementIds:['AC-notes']}]}});
  design=await api('POST',projectRoute+'/design-plan/approve',{revision:design.revision});
  const tasks=[{id:task,title:titles.get(task),status:'todo',requirementIds:['AC-notes'],writePaths:[task+'-result.txt'],acceptance:'Read only this project frozen context, commit its exact controlled task output, and leave primary source unchanged until review.'}];
  const plan=await api('POST',projectRoute+'/plan',{expectedRevision:0,expectedSpecificationRevision:spec.revision,tasks});
  const gitReview=await api('GET',projectRoute+'/git');await api('POST',projectRoute+'/git/commit',{message:'Fixture seed: approved swarm task graph',expectedReviewDigest:gitReview.reviewDigest});
  const branches=(await api('GET',workspace+'/v1/projects/'+project.id+'/branches')).branches;const branch=branches.find(row=>row.name==='main')??branches[0];assert(branch);
  const worktrees=(await api('GET',workspace+'/v1/projects/'+project.id+'/worktrees?branchId='+encodeURIComponent(branch.id))).worktrees;const worktree=worktrees.find(row=>row.isPrimary)??worktrees[0];assert(worktree);
  const selection={projectId:project.id,briefRevision:spec.briefRevision,specificationRevision:spec.revision,designPlanRevision:design.revision,planRevision:plan.revision,taskIds:tasks.map(item=>item.id)};
  const runInput={worktreeId:worktree.id,harness:'codex',model:'swarm-mechanics-fixture',permissionMode:'bypass',agentConnectionId:connectionID,outcome:'preview',implementationContext:selection};
  run=(await api('POST',projectRoute+'/runs',{...runInput,idempotencyKey:'create-installed-swarm-fixture'},201)).run;assert.equal(run.input.route.permissionMode,'bypass');
  const canonicalBefore=Object.fromEntries(canonicalNames.map(name=>[name,sha(readFileSync(join(project.path,name)))])),initialHead=execFileSync('git',['rev-parse','HEAD'],{cwd:project.path,encoding:'utf8'}).trim();
   projectStates.push({owner:project,route:projectRoute,run,task,index,initialHead,canonicalBefore,canonicalLuci:readFileSync(join(project.path,'LUCI.md')),goal:'Prove '+task+' uses its own project context and reviewed integration under a shared worker budget'});
  }

}
