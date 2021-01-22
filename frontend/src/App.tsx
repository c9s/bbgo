import React from 'react';
import cx from "classnames";
import './App.css';

function App() {
  const [toggle, setToggle] = React.useState<boolean>(false)
  console.log("toggle", toggle)

  const buttonClasses = toggle
    ? "bg-red-500 hover:bg-red-500"
    : "bg-blue-500 hover:bg-blue-500"
  return (
    <div className="bg-gray-200 flex items-center justify-center h-screen">
      <button
        className={`p-3 rounded-sm ${buttonClasses}`}
        onClick={() => setToggle(!toggle)}
      >
        Toggle
      </button>
    </div>
  )
}

export default App;
