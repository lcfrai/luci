import { expect, it } from 'vitest';
import { parseActualCost, validOutcomeCosts } from './outcomeCosts';

it('preserves actual bill precision without floating rounding or inferred pricing',()=>{
 expect(parseActualCost('29.500001')).toBe('29500001');
 expect(parseActualCost('0')).toBe('0');
 expect(parseActualCost('0.000001')).toBe('1');
 expect(parseActualCost('0.0000001')).toBeNull();
 expect(parseActualCost('1e3')).toBeNull();
 expect(parseActualCost('-0.01')).toBeNull();
});
it('keeps missing bill and usage data distinct from zero charges',()=>{
 const value={projectId:'p-one',revision:0,rows:[],totals:[],omittedRows:0,attribution:'operator-reconciled-retained-window',providerVerified:false,inputTokens:null,outputTokens:null};
 expect(validOutcomeCosts(value,'p-one')).toBe(true);
 expect(validOutcomeCosts({...value,inputTokens:0},'p-one')).toBe(false);
 expect(validOutcomeCosts({...value,providerVerified:true},'p-one')).toBe(false);
 expect(validOutcomeCosts({...value,totals:[{currency:'USD',amountMicros:'0',receiptRows:0}]},'p-one')).toBe(false);
});
