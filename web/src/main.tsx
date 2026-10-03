import "./theme.css";
import { useGSAP } from "@gsap/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import gsap from "gsap";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { App } from "./App";

gsap.registerPlugin(useGSAP);

const queryClient = new QueryClient();
const root = document.getElementById("root");
if (!root) {
  throw new Error("missing root");
}

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
