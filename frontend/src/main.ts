import { File } from "../bindings/github.com/bh90210/super/api";
import { Events } from "@wailsio/runtime";

// Controls.
const playButton = document.getElementById("play")! as HTMLButtonElement;
const previousButton = document.getElementById(
  "previous"
)! as HTMLButtonElement;
const nextButton = document.getElementById("next")! as HTMLButtonElement;
const progressBar = document.getElementById("progress")! as HTMLSpanElement;
const indicatorBar = document.getElementById("indicator")! as HTMLSpanElement;
const timeElement = document.getElementById("time")! as HTMLLabelElement;
const volumeBar = document.getElementById("volume")! as HTMLInputElement;
const volumeMute = document.getElementById("mute")! as HTMLLabelElement;
const volumeMax = document.getElementById("max")! as HTMLLabelElement;
const statusBarLeft = document.getElementById(
  "status-left"
)! as HTMLParagraphElement;
const statusBarCenter = document.getElementById(
  "status-center"
)! as HTMLParagraphElement;
const statusBarRight = document.getElementById(
  "status-right"
)! as HTMLParagraphElement;

const list = document.getElementById("list")! as HTMLTableElement;

Events.Emit({ name: "ready", data: "" })
  .then(() => {
    console.log("ready");
  })
  .catch((err: Error) => {
    console.log(err);
  });

previousButton.addEventListener("click", () => {
  Events.Emit({ name: "front.previous", data: "" });
});

nextButton.addEventListener("click", () => {
  Events.Emit({ name: "front.next", data: "" });
});

indicatorBar.addEventListener("click", (event: MouseEvent) => {
  Events.Emit({ name: "front.progress", data: event.offsetX });
});

Events.On("segmented", (message: { data: any }) => {
	indicatorBar.classList.toggle("segmented");
});

playButton.addEventListener("click", () => {
  Events.Emit({ name: "front.play.pause", data: "" });
});

volumeBar.addEventListener("mouseup", () => {
  Events.Emit({ name: "front.volume.set", data: volumeBar.value });
});

volumeMute.addEventListener("click", () => {
  Events.Emit({ name: "front.volume.mute", data: "" })
    .then(() => {
      // console.log(result);
    })
    .catch((err: Error) => {
      console.log(err);
    });
});

volumeMax.addEventListener("click", () => {
  Events.Emit({ name: "front.volume.max", data: "" })
    .then(() => {
      // console.log(result);
    })
    .catch((err: Error) => {
      console.log(err);
    });
});

Events.On("status.left", (message: { data: any }) => {
  statusBarLeft.innerText = message.data;
});

Events.On("status.center", (message: { data: any }) => {
  statusBarCenter.innerText = message.data;
});

Events.On("status.right", (message: { data: any }) => {
  statusBarRight.innerText = message.data;
});

Events.On("play.pause", (message: { data: any }) => {
  playButton.textContent = message.data;
});

Events.On("play.pause.deactivate", (message: { data: any }) => {
  playButton.disabled = message.data[0];
});

Events.On("time", (message: { data: any }) => {
  timeElement.innerText = message.data;
});

Events.On("progress.bar", (message: { data: any }) => {
  progressBar.setAttribute("style", `width: ${message.data}%`);
});

Events.On("previous", (message: { data: any }) => {
  previousButton.disabled = message.data[0];
});

Events.On("next", (message: { data: any }) => {
  nextButton.disabled = message.data[0];
});

Events.On("volume.set", (message: { data: any }) => {
  volumeBar.value = message.data;
});

Events.On("list", (entries: { data: File }) => {
  var body = list.getElementsByTagName("tbody")[0];

  entries.data[0].reverse().forEach((entry: File) => {
    var row = body.insertRow(0);

    row.addEventListener("dblclick", () => {
      Events.Emit({ name: "front.list.play", data: row.rowIndex });
    });

    var cell1 = row.insertCell(0);
    var cell2 = row.insertCell(1);
    var cell3 = row.insertCell(2);
    var cell4 = row.insertCell(3);
    var cell5 = row.insertCell(4);
    cell1.innerHTML =
      `<div class="field-row"><input type="checkbox" id="` +
      row.rowIndex +
      `" disabled/> <label for="` +
      row.rowIndex +
      `"></label></div>`;
    cell2.innerHTML = entry.artist ? entry.artist : '';
    cell3.innerHTML = entry.track ? entry.track : '';
    cell4.innerHTML = entry.album ? entry.album : '';
    cell5.innerHTML = entry.duration ? entry.duration : '';
  });
});
