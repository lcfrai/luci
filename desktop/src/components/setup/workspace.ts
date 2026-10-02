import { detachAllTerminals } from "../../api/attachRegistry";
import { cancelDeferredSessionActions } from "../develop/sessionVerbs";
import { useCockpit } from "../../store/store";
import { initialDataState } from "../../store/slices/dataSlice";
import { initialUiState } from "../../store/slices/uiSlice";

export async function changeWorkspace(
  activate: () => Promise<unknown>,
  reload: () => void = () => window.location.reload(),
) {
  cancelDeferredSessionActions();
  await activate();
  detachAllTerminals();
  useCockpit.setState({
    ...initialDataState,
    ...initialUiState,
    lens: "discover",
    projectDefaults: {},
    branchDefaults: {},
    tiles: {},
    pinned: [],
  });
  // Recreate API consumers, catalogue and poller together; an old in-flight response cannot
  // repopulate a new machine's entities after the document reloads.
  reload();
}
