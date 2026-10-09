import {boundedAttempt} from './acceptance-lifecycle.mjs';

const problem = code => Object.assign(new Error(code), {code});

// A rejected workflow race cannot cancel a factory. Track it before invocation,
// retain its eventual handle, and close it once even if its cleanup phase passed.
export function createStartupScope({assertActive, onFailure = () => {}, onTransition = () => {}, lateCloseTimeoutMs = 5000}) {
  const resources = new Map();
  const changed = row => {try {onTransition({name:row.name,state:row.state,late:row.late,closeTimedOut:row.closeTimedOut});} catch (error) {onFailure(error);}};
  function closedChildren(parent) {
    for (const row of resources.values()) if (row.parent === parent && row.state === 'acquired') {
      row.state = 'closed';row.closedByParent = true;changed(row);closedChildren(row.name);
    }
  }
  async function close(name, timeoutMs = lateCloseTimeoutMs) {
    const row = resources.get(name);
    if (!row) throw problem('startup-resource-not-started');
    if (!row.closing) {
      row.closing = (async () => {
        const handle = await row.ready;
        row.state = 'closing';changed(row);
        try {const result = await row.close(handle);row.state = 'closed';closedChildren(name);changed(row);return result;}
        catch (error) {row.state = 'close-failed';changed(row);throw error;}
      })();
      // A bounded waiter can expire before this underlying close settles.
      void row.closing.catch(() => {});
    }
    try {return await boundedAttempt(() => row.closing, timeoutMs, 'startup-'+name+'-close-deadline-exceeded');}
    catch (error) {
      if (error.code === 'startup-'+name+'-close-deadline-exceeded') {row.closeTimedOut = true;changed(row);}
      throw error;
    }
  }
  return {
    async acquire(name, {start, adopt = () => {}, close: closeHandle, parent}) {
      assertActive();
      if (!/^[a-z][a-z-]{0,31}$/.test(name) || resources.has(name)) throw problem('startup-resource-name-refused');
      if (parent && resources.get(parent)?.state !== 'acquired') throw problem('startup-parent-not-active');
      const row = {name,state:'starting',late:false,closeTimedOut:false,close:closeHandle,parent};
      resources.set(name,row);changed(row);
      row.ready = Promise.resolve().then(() => {assertActive();return start();}).then(handle => {
        row.state = 'acquired';changed(row);return handle;
      }, error => {row.state = 'factory-failed';changed(row);throw error;});
      let handle;
      try {handle = await row.ready;}
      catch (error) {throw error;}
      try {adopt(handle);assertActive();}
      catch (error) {
        row.late = true;changed(row);
        try {await close(name,lateCloseTimeoutMs);} catch (closeError) {onFailure(closeError);}
        throw error;
      }
      return handle;
    },
    close,
    has: name => resources.has(name),
    snapshot: () => [...resources.values()].map(({name,state,late,closeTimedOut,parent,closedByParent}) => ({name,state,late,closeTimedOut,...(parent?{parent,closedByParent:closedByParent===true}:{})})),
  };
}

// This is the service startup sequence used by the installed acceptance drivers.
// Injected factories let regression tests hold the real orchestration at each
// await without starting Docker, a provider, or a detached service process.
export async function startAcceptanceServices({scope,assertActive,startProvider,adoptProvider,closeProvider,freePort,startProxy,adoptProxy,closeProxy,configure,startService}) {
  const provider = await scope.acquire('provider',{start:startProvider,adopt:adoptProvider,close:closeProvider});
  assertActive();const port = await freePort();
  assertActive();const preview = await freePort();
  assertActive();const proxy = await scope.acquire('proxy',{start:startProxy,adopt:adoptProxy,close:closeProxy});
  assertActive();configure({provider,proxy,port,preview});
  assertActive();await startService();
  assertActive();return {provider,proxy,port,preview};
}

export async function startAcceptanceBrowser({scope,assertActive,launch,adoptBrowser,closeBrowser,newContext,installRoutes,newPage,adoptPage}) {
  const browser = await scope.acquire('browser',{start:launch,adopt:adoptBrowser,close:closeBrowser});
  assertActive();const context = await scope.acquire('browser-context',{start:()=>newContext(browser),close:handle=>handle.close(),parent:'browser'});
  assertActive();await installRoutes(context);
  assertActive();const page = await scope.acquire('browser-page',{start:()=>newPage(context),adopt:adoptPage,close:handle=>handle.close(),parent:'browser-context'});
  assertActive();return {browser,context,page};
}
