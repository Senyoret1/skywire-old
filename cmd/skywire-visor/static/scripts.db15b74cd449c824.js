var Terminal=function(){var i,l=function(n,t){var e=t._cursor;setTimeout(function(){n.parentElement&&t._shouldBlinkCursor?(e.style.visibility="visible"===e.style.visibility?"hidden":"visible",l(n,t)):e.style.visibility="visible"},500)},c=!0;return promptInput=function(n,t,e,o){var p=1===e;(i=document.createElement("input")).style.position="absolute",i.style.zIndex="-100",i.style.outline="none",i.style.border="none",i.style.opacity="0",i.style.fontSize="0.2em",n._inputLine.textContent="",n._input.style.display="block",n.html.appendChild(i),l(i,n),t.length&&n.print(3===e?t+" (y/n)":t,!0),i.onblur=function(){n._cursor.style.opacity=0},i.onfocus=function(){i.value=n._inputLine.textContent,n._cursor.style.display="inline",n._cursor.style.opacity=1},n.html.onclick=function(){i.focus()},i.onkeydown=function(s){37===s.which||39===s.which||38===s.which||40===s.which||9===s.which?s.preventDefault():p&&13!==s.which&&setTimeout(function(){n._inputLine.textContent=i.value},1)},i.onkeyup=function(s){if(3===e||13===s.which){n._input.style.display="none";var h=i.value;p&&n.print(h),n.html.removeChild(i),i=void 0,"function"==typeof o&&o(3===e?"Y"===h.toUpperCase()[0]:h)}},c?(c=!1,setTimeout(function(){i.focus()},50)):i.focus()},function(n){this.html=document.createElement("div"),this.html.className="Terminal","string"==typeof n&&(this.html.id=n),this._innerWindow=document.createElement("div"),this._output=document.createElement("p"),this._inputLine=document.createElement("span"),this._cursor=document.createElement("span"),this._input=document.createElement("p"),this._shouldBlinkCursor=!0,this.print=function(t,e){var o=document.createElement("div");o.innerHTML=t,this._output.appendChild(o),e&&(o.style.color="#00bd00")},this.input=function(t,e){promptInput(this,t,1,e)},this.changeInputContent=function(t){if(i&&this._inputLine)try{i.value=t,this._inputLine.textContent=t}catch(e){}},this.getInputContent=function(){return i?i.value:""},this.hasFocus=function(){return i&&document.activeElement===i},this.password=function(t,e){promptInput(this,t,2,e)},this.confirm=function(t,e){promptInput(this,t,3,e)},this.clear=function(){this._output.innerHTML=""},this.sleep=function(t,e){setTimeout(e,t)},this.setTextSize=function(t){this._output.style.fontSize=t,this._input.style.fontSize=t},this.setTextColor=function(t){this.html.style.color=t,this._cursor.style.background=t},this.setBackgroundColor=function(t){this.html.style.background=t},this.setWidth=function(t){this.html.style.width=t},this.setHeight=function(t){this.html.style.height=t},this.blinkingCursor=function(t){t=t.toString().toUpperCase(),this._shouldBlinkCursor="TRUE"===t||"1"===t||"YES"===t},this._input.appendChild(this._inputLine),this._input.appendChild(this._cursor),this._innerWindow.appendChild(this._output),this._innerWindow.appendChild(this._input),this.html.appendChild(this._innerWindow),this.setBackgroundColor("black"),this.setTextColor("white"),this.setTextSize("1em"),this.setWidth("100%"),this.setHeight("100%"),this.html.style.fontFamily="Monaco, Courier",this.html.style.margin="0",this._innerWindow.style.padding="10px",this._input.style.margin="0",this._output.style.margin="0",this._cursor.style.background="white",this._cursor.innerHTML="C",this._cursor.style.display="none",this._input.style.display="none"}}();