import { render } from "preact";
import { App } from "./App";
import "./styles/app.css";
import { lang } from "./lib/i18n";

document.documentElement.lang = lang;

render(<App />, document.getElementById("app")!);
