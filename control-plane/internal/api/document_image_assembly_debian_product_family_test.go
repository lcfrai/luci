package api

import (
 "bytes"
 "context"
 "encoding/json"
 "reflect"
 "strings"
 "sync"
 "testing"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
)

// Synthetic selected metadata ONLY. No expected production pins, host proof,
// original completion, native process, signature or eligible profile is minted.
func productFamilyFixture(t *testing.T)(assemblyDebianProductProfilePins,[]byte){
 t.Helper();ref:=func(role,digit string)assemblyDebianObjectRef{return assemblyDebianObjectRef{role,strings.Repeat(digit,64),1}}
 p:=assemblyDebianProductProfilePins{Profile:ref("product-profile","1"),SourceReceipt:ref("producer-source-receipt","2"),Entry:ref("projection-entry","3"),Projection:assemblyDebianObjectRef{"projection-source",assemblyDebianProjectionSHA,15522},Python:ref("cp-python","4"),GPG:ref("gpg","5"),GPGV:ref("gpgv","6")}
 config:=assemblyDebianProductConfigV1{Schema:assemblyDebianProductConfigSchema,Protocol:assemblyDebianProductProtocol,TrustPolicySHA256:assemblyDebianPolicySourceSHA,ProjectionSHA256:assemblyDebianProjectionSHA,DependencyLockSHA256:assemblyDebianLockSHA,AcquisitionManifestSHA256:assemblyDebianManifestSHA,SnapshotSourceRef:expectedAssemblyDebianHistoricalAnchor().SnapshotSourceRef,ProjectionConfig:json.RawMessage(`{}`)}
 data,e:=json.Marshal(config);if e!=nil{t.Fatal(e)};p.Config=assemblyDebianObjectRef{"producer-config",assemblyCatalogDigest(data),int64(len(data))};return p,data
}
func productProducerFixture(p assemblyDebianProductProfilePins)(assemblyDebianProducerV1,assemblyDebianProducerPins){
 producer:=assemblyDebianProducerV1{ProductOwner:&assemblyDebianProductOwnerV1{Protocol:assemblyDebianProductProtocol,Profile:p.Profile,Receipt:assemblyDebianObjectRef{"product-native-owner-receipt",strings.Repeat("7",64),1},Entry:p.Entry,Python:p.Python},Profile:assemblyDebianProductRawSchema,SourceReceipt:p.SourceReceipt,Config:p.Config,TypedOutput:assemblyDebianObjectRef{"producer-output",strings.Repeat("8",64),1},ProjectionSHA256:p.Projection.SHA256,GPGSHA256:p.GPG.SHA256,GPGVSHA256:p.GPGV.SHA256}
 return producer,assemblyDebianProducerPins{Product:&p,SourceReceipt:p.SourceReceipt,Config:p.Config,ProjectionSHA256:p.Projection.SHA256,GPGSHA256:p.GPG.SHA256,GPGVSHA256:p.GPGV.SHA256}
}
func TestDebianProductFamilyNormalizesIndependentSelectedConfig(t *testing.T){
 source,old:=assemblyDebianSourceFixture(t);pins,config:=productFamilyFixture(t)
 legacy,e:=normalizeAssemblyDebianPolicy(source,old,"family-fixture");if e!=nil{t.Fatal(e)}
 product,e:=normalizeAssemblyDebianProductPolicy(source,config,pins,"family-fixture");if e!=nil{t.Fatal(e)}
 if product.ProducerProtocol!=assemblyDebianProductProtocol||product.ProducerProfile==nil||*product.ProducerProfile!=pins.Profile||product.ProducerConfig!=pins.Config||!reflect.DeepEqual(product.Roots,legacy.Roots)||!reflect.DeepEqual(product.Releases,legacy.Releases)||product.HistoricalAnchor!=legacy.HistoricalAnchor{t.Fatal("product config changed finite official facts or claimed legacy config")}
 if _,e=normalizeAssemblyDebianPolicy(source,config,"family-fixture");e==nil{t.Fatal("product config entered legacy normalizer")}
 if _,e=normalizeAssemblyDebianProductPolicy(source,old,pins,"family-fixture");e==nil{t.Fatal("diagnostic config entered product normalizer")}
}
func TestDebianProductFamilySingleOwnerReceiptRejectsMixedOrUnknownArms(t *testing.T){
 p,_:=productFamilyFixture(t);value,pins:=productProducerFixture(p);if e:=checkAssemblyDebianProducer(value,pins);e!=nil{t.Fatal(e)}
 for _,mutate:=range []func(*assemblyDebianProducerV1){func(v *assemblyDebianProducerV1){v.Profile="luci-retained-debian-signature-input-evidence-v2"},func(v *assemblyDebianProducerV1){v.InnerReceipt=assemblyDebianObjectRef{"inner-receipt",strings.Repeat("a",64),1}},func(v *assemblyDebianProducerV1){v.OuterReceipt=assemblyDebianObjectRef{"outer-receipt",strings.Repeat("a",64),1}},func(v *assemblyDebianProducerV1){v.ProductOwner=nil},func(v *assemblyDebianProducerV1){v.ProductOwner.Protocol="unknown-owner"},func(v *assemblyDebianProducerV1){v.ProductOwner.Receipt.Role="outer-receipt"},func(v *assemblyDebianProducerV1){v.ProductOwner.Profile.SHA256=strings.Repeat("b",64)}}{
  bad:=value;copyOwner:=*value.ProductOwner;bad.ProductOwner=&copyOwner;mutate(&bad);if checkAssemblyDebianProducer(bad,pins)==nil{t.Fatal("mixed/unknown/substituted producer arm accepted")}
 }
 legacyPins:=pins;legacyPins.Product=nil;if checkAssemblyDebianProducer(value,legacyPins)==nil{t.Fatal("product receipt accepted as legacy")}
}
func TestDebianProductFamilyPinsRejectDiagnosticAndMissingExpectations(t *testing.T){
 p,_:=productFamilyFixture(t)
 for _,mutate:=range []func(*assemblyDebianProductProfilePins){func(p *assemblyDebianProductProfilePins){p.SourceReceipt.SHA256=assemblyDebianSourceReceiptSHA},func(p *assemblyDebianProductProfilePins){p.Config.SHA256=assemblyDebianConfigSHA},func(p *assemblyDebianProductProfilePins){p.Profile=assemblyDebianObjectRef{}},func(p *assemblyDebianProductProfilePins){p.GPG.SHA256=strings.Repeat("0",64)},func(p *assemblyDebianProductProfilePins){p.Projection.Bytes++}}{bad:=p;mutate(&bad);if validAssemblyDebianProductPins(bad){t.Fatal("diagnostic/missing expected product ref accepted")}}
}
func TestDebianProductFamilyClosedConfigRefusesUnknownAndWrongBindings(t *testing.T){
 source,_:=assemblyDebianSourceFixture(t);pins,data:=productFamilyFixture(t);var base assemblyDebianProductConfigV1;if json.Unmarshal(data,&base)!=nil{t.Fatal("fixture")}
 for _,mutate:=range []func(*assemblyDebianProductConfigV1){func(c *assemblyDebianProductConfigV1){c.Schema="diagnostic-config"},func(c *assemblyDebianProductConfigV1){c.Protocol="unknown"},func(c *assemblyDebianProductConfigV1){c.TrustPolicySHA256=strings.Repeat("a",64)},func(c *assemblyDebianProductConfigV1){c.SnapshotSourceRef.Bytes++},func(c *assemblyDebianProductConfigV1){c.ProjectionConfig=json.RawMessage(`[]`)}}{bad:=base;mutate(&bad);raw,_:=json.Marshal(bad);expected:=pins;expected.Config=assemblyDebianObjectRef{"producer-config",assemblyCatalogDigest(raw),int64(len(raw))};if _,e:=normalizeAssemblyDebianProductPolicy(source,raw,expected,"fixture");e==nil{t.Fatal("wrong closed product config accepted")}}
 unknown:=append(append([]byte{},data[:len(data)-1]...),[]byte(`,"unknown":1}`)...);pins.Config=assemblyDebianObjectRef{"producer-config",assemblyCatalogDigest(unknown),int64(len(unknown))};if _,e:=normalizeAssemblyDebianProductPolicy(source,unknown,pins,"fixture");e==nil{t.Fatal("unknown config field accepted")}
}
func TestDebianProductFamilyCrossSummaryUsesValueEqualityAndSamePolicyFamily(t *testing.T){
 source,_:=assemblyDebianSourceFixture(t);selected,config:=productFamilyFixture(t);policy,e:=normalizeAssemblyDebianProductPolicy(source,config,selected,"family-fixture");if e!=nil{t.Fatal(e)}
 _,keys,signatures,oldPins,now:=assemblyDebianSummaryFixture(t);producer,pins:=productProducerFixture(selected);pins.SignedObjects=oldPins.SignedObjects
 keys.Producer=producer;keys.RootPolicy,_=assemblyDebianRecordRef("official-root-policy",policy);signatures.RootPolicy=keys.RootPolicy;signatures.KeyObservations,_=assemblyDebianRecordRef("key-observations",keys)
 raw,_:=json.Marshal(producer);if json.Unmarshal(raw,&signatures.Producer)!=nil{t.Fatal("producer fixture")}
 if signatures.Producer.ProductOwner==keys.Producer.ProductOwner{t.Fatal("fixture did not create distinct equivalent owner pointers")}
 if e=checkAssemblyDebianSignatures(signatures,policy,keys,pins,now);e!=nil{t.Fatal(e)}
 legacy,_,_,_,_:=assemblyDebianSummaryFixture(t);if checkAssemblyDebianKeys(keys,legacy,pins,now)==nil{t.Fatal("legacy policy accepted product producer")}
 badPins:=pins;badPins.Product=nil;if checkAssemblyDebianKeys(keys,policy,badPins,now)==nil{t.Fatal("product policy accepted legacy expected pins")}
}
func TestDebianProductFamilyMetadataCannotEnableActualReservation(t *testing.T){
 p:=producerProfileFixture(t);for i:=range p.Files{switch p.Files[i].Role{case "producer-config":p.Files[i].SHA256=strings.Repeat("d",64);case "projection-source":p.Files[i].Bytes=15522}}
 raw,_:=canonicalDebianProducerProfile(p);p.SHA256=assemblyCatalogDigest(raw)
 if _,e:=validateDebianProductProfile(p);e!=nil{t.Fatal(e)}
 if _,e:=validateDebianProducerProfile(p);e==nil{t.Fatal("legacy tool-domain shape validator silently changed family")}
 if _,e:=debianProducerProfileBeforeReservation(p);e==nil{t.Fatal("shape-valid profile supplied missing package/current-host/copy authority")}
 if _,e:=newAssemblyDebianProductDomains(context.Background(),nil,nil,time.Now().Unix());e==nil{t.Fatal("metadata substituted for original completion")}
}

// Frozen legacy field layout: additive omitted family fields must not change
// canonical bytes or the exact existing record digest on legacy serialization.
func TestDebianProductFamilyLegacyProducerCanonicalBytesRemainUnchanged(t *testing.T){
 _,keys,_,_,_:=assemblyDebianSummaryFixture(t);v:=keys.Producer
 old:=struct{Profile string `json:"profile"`;SourceReceipt assemblyDebianObjectRef `json:"sourceReceipt"`;Config assemblyDebianObjectRef `json:"config"`;TypedOutput assemblyDebianObjectRef `json:"typedOutput"`;InnerReceipt assemblyDebianObjectRef `json:"innerReceipt"`;OuterReceipt assemblyDebianObjectRef `json:"outerReceipt"`;Projection string `json:"projectionSha256"`;GPG string `json:"gpgSha256"`;GPGV string `json:"gpgvSha256"`}{v.Profile,v.SourceReceipt,v.Config,v.TypedOutput,v.InnerReceipt,v.OuterReceipt,v.ProjectionSHA256,v.GPGSHA256,v.GPGVSHA256}
 before,_:=json.Marshal(old);after,_:=json.Marshal(v);if !bytes.Equal(before,after)||assemblyCatalogDigest(before)!=assemblyCatalogDigest(after){t.Fatal("omitted product arm rewrote legacy canonical producer")}
 policy,_,_,_,_:=assemblyDebianSummaryFixture(t);policyBytes,_:=json.Marshal(policy);if bytes.Contains(policyBytes,[]byte(`"producerProtocol"`))||bytes.Contains(policyBytes,[]byte(`"producerProfile"`)){t.Fatal("legacy policy gained a product discriminator")}
}

type productFamilyBlockedTransferGuard struct {assemblyDebianFakeGuard;entered,release chan struct{};late *assemblyDebianFakeGuard}
func(g *productFamilyBlockedTransferGuard)Transfer(context.Context)(backend.DocumentImageAssemblyEvidenceGuard,error){g.mu.Lock();g.valid=false;g.mu.Unlock();close(g.entered);<-g.release;return g.late,nil}
func TestDebianProductFamilyLateTransferRetainsRetirementOwnership(t *testing.T){
 late:=&assemblyDebianFakeGuard{valid:true};output:=&productFamilyBlockedTransferGuard{assemblyDebianFakeGuard:assemblyDebianFakeGuard{valid:true},entered:make(chan struct{}),release:make(chan struct{}),late:late};inputs:=&assemblyDebianFakeGuard{valid:true}
 var releaseOnce sync.Once;release:=func(){releaseOnce.Do(func(){close(output.release)})};defer release()
 r:=&assemblyDebianOwnedRun{phase:"captured",product:&debianProductProofBinding{},guard:output,inputGuard:inputs};done:=make(chan error,1)
 go func(){_,e:=finishAssemblyDebianProductCandidate(context.Background(),r,&assemblyDebianDomainCandidate{});done<-e}()
 select{case<-output.entered:case<-time.After(time.Second):t.Fatal("synthetic guard transfer not entered")}
 retired:=make(chan struct{});go func(){_ = r.retire();close(retired)}();select{case<-retired:case<-time.After(time.Second):release();t.Fatal("guard IO held the run retirement mutex")};release()
 select{case e:=<-done:if e==nil{t.Fatal("retirement became promotion success")};case<-time.After(time.Second):t.Fatal("late original token was not joined")}
 late.mu.Lock();closed:=late.closed;late.mu.Unlock();r.mu.Lock();phase:=r.phase;r.mu.Unlock();if !closed||phase!="closed"{t.Fatal("late returned token was lost instead of retired")}
}
