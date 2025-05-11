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

const searchInput = document.getElementById(
  "search-input"
)! as HTMLInputElement;
const searchButton = document.getElementById(
  "search-button"
)! as HTMLButtonElement;

const duploadButton = document.getElementById("dupload")! as HTMLButtonElement;

const minimizeButton = document.getElementById(
  "minimize"
)! as HTMLButtonElement;

const closeButton = document.getElementById("close")! as HTMLButtonElement;

const maximizeButton = document.getElementById(
  "maximize"
)! as HTMLButtonElement;

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

Events.On("segmented.off", (message: { data: any }) => {
  indicatorBar.classList.remove("segmented");
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

searchInput.addEventListener("keyup", (event: KeyboardEvent) => {
  if (searchInput.value !== "") {
    searchButton.textContent = "Clear";
  } else {
    searchButton.textContent = "Search";
  }

  //   if (event.key === "Enter") {
  Events.Emit({ name: "front.search.query", data: searchInput.value })
    .then(() => {
      // console.log(result);
    })
    .catch((err: Error) => {
      console.log(err);
    });
  //   }
});

searchButton.addEventListener("click", () => {
  if (searchButton.textContent === "Clear") {
    searchInput.value = "";
    searchButton.textContent = "Search";
  }

  Events.Emit({ name: "front.search.button", data: searchInput.value })
    .then(() => {
      // console.log(result);
    })
    .catch((err: Error) => {
      console.log(err);
    });
});

duploadButton.addEventListener("click", () => {
  Events.Emit({ name: "front.dupload", data: "" })
    .then(() => {
      // console.log(result);
    })
    .catch((err: Error) => {
      console.log(err);
    });
});

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
  var rows = body.rows;

  if (rows.length > 0) {
	rows[0].scrollIntoView({behavior: "smooth", block: "start", inline: "nearest"});
    for (var i = rows.length - 1; i >= 0; i--) {
      body.removeChild(rows[i]);
    }
  }

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
      `<div><input type="checkbox" id="` +
      row.rowIndex +
      `" disabled/> <label for="` +
      row.rowIndex +
      `"></label></div>`;
    cell2.innerHTML =
      `<div class="d-inline-block text-truncate" style="max-width: 200px;">` +
      (entry.artist ? entry.artist : "") +
      `</div>`;
    cell3.innerHTML =
      `<div class="d-inline-block text-truncate" style="max-width: 200px;">` +
      (entry.track ? entry.track : "") +
      `</div>`;
    cell4.innerHTML =
      `<div class="d-inline-block text-truncate" style="max-width: 200px;">` +
      (entry.album ? entry.album : "") +
      `</div>`;
    cell5.innerHTML =
      `<div class="d-inline-block text-truncate" style="max-width: 200px;">` +
      (entry.duration ? entry.duration : "") +
      `</div>`;
  });
});
