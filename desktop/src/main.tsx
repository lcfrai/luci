// The token layer and the document ground load FIRST, before any component stylesheet.
// Vite emits CSS in module-graph order, so importing this after "./App" would put every
// component sheet ahead of the reset that is supposed to sit under it.
import "./styles/app.css";
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
