import { Events } from "@wailsio/runtime";

const minimizeButton = document.getElementById(
  "minimize"
)! as HTMLButtonElement;

const closeButton = document.getElementById("close")! as HTMLButtonElement;

const maximizeButton = document.getElementById(
  "maximize"
)! as HTMLButtonElement;

minimizeButton.addEventListener("click", () => {
  Events.Emit({ name: "front.minimize", data: "" })
    .then(() => {
      // console.log(result);
    })
    .catch((err: Error) => {
      console.log(err);
    });
});

maximizeButton.addEventListener("click", () => {
  Events.Emit({ name: "front.maximize", data: "" })
    .then(() => {
      // console.log(result);
    })
    .catch((err: Error) => {
      console.log(err);
    });
});

closeButton.addEventListener("click", () => {
  Events.Emit({ name: "front.close", data: "" })
    .then(() => {
      // console.log(result);
    })
    .catch((err: Error) => {
      console.log(err);
    });
});
