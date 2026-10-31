import { useState } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it } from 'vitest';
import { InferenceHealthEditor } from './InferenceHealthEditor';
import type { BlueprintHealth } from '../../api/deploymentBlueprint';

afterEach(cleanup);
function Harness(){const [value,setValue]=useState<BlueprintHealth>();return <><InferenceHealthEditor value={value} port={8000} onChange={setValue}/><output data-testid="health">{JSON.stringify(value??null)}</output></>;}

it('requires an explicit image health path and keeps probes on the declared port',async()=>{
 const user=userEvent.setup();render(<Harness/>);await user.click(screen.getByText('Startup and readiness checks'));
 await user.click(screen.getByRole('checkbox',{name:'Configure readiness probe'}));
 expect((screen.getByRole('textbox',{name:'Readiness HTTP path'}) as HTMLInputElement).value).toBe('');
 await user.type(screen.getByRole('textbox',{name:'Readiness HTTP path'}),'/ready');
 const value=JSON.parse(screen.getByTestId('health').textContent??'null');expect(value.readiness.path).toBe('/ready');expect(value.readiness.port).toBeUndefined();
 expect(screen.getByText(/container port 8000/)).not.toBeNull();
});

it('removes HTTP-only fields when TCP is selected and removes optional health when all probes are off',async()=>{
 const user=userEvent.setup();render(<Harness/>);await user.click(screen.getByText('Startup and readiness checks'));await user.click(screen.getByRole('checkbox',{name:'Configure startup probe'}));
 await user.type(screen.getByRole('textbox',{name:'Startup HTTP path'}),'/start');await user.selectOptions(screen.getByRole('combobox',{name:'Startup protocol'}),'tcp');
 expect(screen.queryByRole('textbox',{name:'Startup HTTP path'})).toBeNull();
 expect(JSON.parse(screen.getByTestId('health').textContent??'null').startup.path).toBeUndefined();
 await user.click(screen.getByRole('checkbox',{name:'Configure startup probe'}));expect(screen.getByTestId('health').textContent).toBe('null');
});
