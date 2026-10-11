// Use the actual queue disclosure; preserve collapsed-by-default product behavior.
export async function openQueueControls(page, assertActive = () => {}) {
  assertActive();
  const region = page.getByRole('region', {name: 'Background task queue', exact: true});
  const disclosure = region.locator('details.swarm-queue-controls');
  await disclosure.waitFor({state: 'attached'}); assertActive();
  const open = await disclosure.evaluate(element => element.open); assertActive();
  if (!open) {
    await disclosure.locator(':scope > summary').click(); assertActive();
  }
  await region.getByRole('button', {name: 'Refresh queue', exact: true}).waitFor({state: 'visible'}); assertActive();
  return region;
}
