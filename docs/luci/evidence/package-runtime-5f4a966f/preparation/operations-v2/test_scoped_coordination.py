"""Pure controls: no proc access, tool hashing, copying or moving."""
import copy,unittest,io,types
from unittest.mock import patch
from operation_common import validate_coordination,require_idle,process_scan,Refusal
SOURCES={'operation_common.py':'a'*64,'relocate_tools.py':'b'*64}
def receipt():return {'version':2,'scope':'cloud054-exclusive-tools','exclusive':True,'activeOperations':[],'bootID':'fixture-boot','monotonicNS':100,'expiresMonotonicNS':600_000_000_100,'ownerPID':7,'ownerStartTicks':'70','operationSources':SOURCES,'acceptedUnknownProcesses':[{'pid':42,'startTicks':'420'}]}
class ScopedControls(unittest.TestCase):
 def test_exact_unknown_allowed_new_reused_missing_unstable_refused(self):
  c=receipt();good={'pid':42,'startTicks':'420','identityStable':True};self.assertEqual(require_idle({'busy':[],'unknown':[good]},c)['acceptedObservedUnknowns'],[good])
  for row in [{'pid':43,'startTicks':'420','identityStable':True},{'pid':42,'startTicks':'421','identityStable':True},{'pid':42,'startTicks':None,'identityStable':False},{'pid':42,'startTicks':'420','identityStable':False}]:
   with self.subTest(row=row),self.assertRaises(Refusal):require_idle({'busy':[],'unknown':[row]},c)
 def test_busy_reference_never_allowed_even_for_explicit_identity(self):
  for c in [receipt(),dict(receipt(),acceptedUnknownProcesses=[])]:
   with self.subTest(c=c),self.assertRaises(Refusal):require_idle({'busy':[{'pid':42,'startTicks':'420','references':['exe']}],'unknown':[]},c)
  class Entry:
   def __init__(self,name):self.name=name
   def read_text(self):return '42 (fixture) S '+'0 '*18+'420'
   def open(self,mode):
    if self.name=='maps':raise unreadable_error('inert unreadable maps')
    return io.BytesIO(b'/tool/path/bin/rustc\0')
  class PID:
   name='42'
   def __truediv__(self,name):return Entry(name)
  fake=types.SimpleNamespace(iterdir=lambda:[PID()])
  for unreadable_error in [PermissionError,FileNotFoundError]:
   with patch('operation_common.os.readlink',side_effect=lambda p:'/tool/path/bin/rustc' if p.name=='exe' else '/unrelated'):
    scan=process_scan(['/tool/path'],proc=fake)
   self.assertEqual(scan['busy'][0]['references'],['argument-path','exe'])
   if unreadable_error is PermissionError:self.assertEqual(scan['unknown'][0]['startTicks'],'420')
   with self.assertRaises(Refusal):require_idle(scan,receipt())
 def test_explicit_expiry_live_owner_and_source_scope_bounded(self):
  o=receipt();self.assertIs(validate_coordination(o,o['scope'],200,'fixture-boot',lambda pid:'70',SOURCES),o)
  cases=[({'expiresMonotonicNS':600_000_000_101},200,lambda pid:'70',SOURCES),({},600_000_000_101,lambda pid:'70',SOURCES),({},99,lambda pid:'70',SOURCES),({},200,lambda pid:'71',SOURCES),({'scope':'cloud054-exclusive-docker'},200,lambda pid:'70',SOURCES),({'operationSources':{}},200,lambda pid:'70',SOURCES),({'expiresMonotonicNS':None},200,lambda pid:'70',SOURCES)]
  for change,now,owner,sources in cases:
   c=copy.deepcopy(o);c.update(change)
   with self.subTest(change=change,now=now),self.assertRaises(Refusal):validate_coordination(c,c['scope'],now,'fixture-boot',owner,sources)
  def absent(_):raise ProcessLookupError('inert absent owner')
  with self.assertRaises(ProcessLookupError):validate_coordination(o,o['scope'],200,'fixture-boot',absent,SOURCES)
if __name__=='__main__':unittest.main()
