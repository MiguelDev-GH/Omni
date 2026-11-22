import type { Target } from "../api/targets";
import { PortField } from "./common";

export default function NiktoOptions(props: {
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
  target: Target;
}) {
  return <PortField id="nikto-porta" {...props} />;
}
