import { expect, it, vi } from 'vitest';
import ts from 'typescript';
import { browserDataClientSource } from './DataClientSetup';

it('generated Supabase client includes team browser credentials only for its configured data origin',async()=>{
  const configured='http://data.example.localhost:8731';
  const fetcher=vi.fn().mockResolvedValue(new Response('{}'));
  const source=ts.transpileModule(browserDataClientSource(configured,'public-anon'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;
  const exports: {supabase?: {fetch:typeof fetch}}={};
  const require=(id:string)=>{
    expect(id).toBe('@supabase/supabase-js');
    return {createClient:(url:string,key:string,options:{global:{fetch:typeof fetch}})=>{
      expect(url).toBe(configured);expect(key).toBe('public-anon');return options.global;
    }};
  };
  new Function('exports','require','window','fetch',source)(exports,require,{location:{href:'http://app.example.localhost:8731/'}},fetcher);
  await exports.supabase!.fetch(configured+'/rest/v1/notes',{headers:{apikey:'public-anon'}});
  expect(fetcher).toHaveBeenLastCalledWith(configured+'/rest/v1/notes',{headers:{apikey:'public-anon'},credentials:'include'});
  await exports.supabase!.fetch('https://other.example/endpoint',{credentials:'omit'});
  expect(fetcher).toHaveBeenLastCalledWith('https://other.example/endpoint',{credentials:'omit'});
});
