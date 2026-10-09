"""Pure exact760 space arithmetic. Never observes or mutates the host."""
EXPORT_FLOOR=3905250841
RUNTIME_FLOOR=6736759858
GZIP_CAP=1288490188 # floor(1.2 GiB), a hard refusal bound, never truncation
RUST=620087163
PWSH=192479038
HISTORICAL425=1013359941
HISTORICAL333=1013367768
GUARD=64*1024*1024

def admission(image_size,root_free,tmp_free):
 if type(image_size)is not int or image_size<=0:raise ValueError('invalid-image-size')
 required=image_size+1342177280+268435456
 if required!=EXPORT_FLOOR:raise ValueError('exact760-size-or-budget-changed')
 if root_free<EXPORT_FLOOR+GZIP_CAP or tmp_free<EXPORT_FLOOR:raise ValueError('unchanged-export-floor-plus-output-budget-refused')
 return {'perFilesystemExportFloorBytes':EXPORT_FLOOR,'rawLimitBytes':image_size+(256<<20),'gzipLimitBytes':GZIP_CAP,'workspaceRequiredBytes':EXPORT_FLOOR+GZIP_CAP,'tmpRequiredBytes':EXPORT_FLOOR,'scope':'Output cap is an additional budget; overflow fails and preserves incomplete evidence. Daemon staging location is unknown.'}

def forecast(root_free,tmp_free,archive_bytes=GZIP_CAP):
 moved=RUST+PWSH+HISTORICAL425
 return {'input':{'root':root_free,'tmp':tmp_free},'optionalToRoot':{'root':root_free-moved,'tmp':tmp_free+moved},'afterGzipAtBound':{'root':root_free-moved-archive_bytes,'tmp':tmp_free+moved},'optionalReturned':{'root':root_free-archive_bytes,'tmp':tmp_free},'old333HistoricalToTMP':{'root':root_free-archive_bytes+HISTORICAL333,'tmp':tmp_free-HISTORICAL333},'basis':'Logical bytes only, excludes filesystem allocation, receipts, growth and concurrent usage. Actual stage observations always govern.'}
